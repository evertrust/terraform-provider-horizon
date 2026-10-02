//go:build e2e

// nolint
package tests

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"golang.org/x/sync/errgroup"
)

type MongoContainer struct {
	testcontainers.Container
}

type HorizonContainer struct {
	testcontainers.Container
}

type NginxContainer struct {
	testcontainers.Container
	HttpUrl string
}

type HorizonTestInstances struct {
	Nginx   *NginxContainer
	Mongo   *MongoContainer
	Horizon *HorizonContainer
}

func DownHorizonInstance(
	ctx context.Context,
	horizonEnv HorizonTestInstances,
) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	// Save logs to the reports dir for debugging
	logs, _ := horizonEnv.Horizon.Logs(ctx)

	fileBytes, err := io.ReadAll(logs)
	if err != nil {
		return err
	}

	logPath := filepath.Join(cwd, "..", "reports", fmt.Sprintf("horizon-%d.log", os.Getpid()))

	err = os.WriteFile(logPath, fileBytes, 0666)
	if err != nil {
		return err
	}

	if err = horizonEnv.Mongo.Container.Terminate(ctx); err != nil {
		return err
	}
	if err = horizonEnv.Horizon.Container.Terminate(ctx); err != nil {
		return err
	}
	if err = horizonEnv.Nginx.Container.Terminate(ctx); err != nil {
		return err
	}
	return nil
}

func seedConfigFolder(root, horizonVersion string) (string, error) {
	wantMajor, wantMinor, err := parseMajorMinor(horizonVersion)
	if err != nil {
		return "", fmt.Errorf("invalid HRZ_VERSION %q: %w", horizonVersion, err)
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return "", err
	}

	best, bestMajor, bestMinor := "", -1, -1
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		major, minor, err := parseMajorMinor(e.Name())
		if err != nil {
			continue
		}
		if major > wantMajor || (major == wantMajor && minor > wantMinor) {
			continue
		}
		if major > bestMajor || (major == bestMajor && minor > bestMinor) {
			best, bestMajor, bestMinor = e.Name(), major, minor
		}
	}
	if best == "" {
		return "", fmt.Errorf("no seed folder in %s for Horizon %s or any earlier version", root, horizonVersion)
	}
	return filepath.Join(root, best), nil
}

func parseMajorMinor(version string) (int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("expected <major>.<minor>[.<patch>], got %q", version)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

func UpHorizonInstance(ctx context.Context, t *testing.T) (*HorizonTestInstances, error) {
	networkName := os.Getenv("DOCKER_NETWORK")
	if networkName == "" {
		newNetwork, err := network.New(ctx)
		if err != nil {
			return nil, err
		}
		networkName = newNetwork.Name
	}
	mongoVersion := os.Getenv("MONGO_VERSION")
	horizonVersion := os.Getenv("HRZ_VERSION")
	req := testcontainers.ContainerRequest{
		Image:        "mongo:" + mongoVersion,
		ExposedPorts: []string{"27017/tcp"},
		WaitingFor: wait.ForAll(
			// Not taking the w from waiting since it is case dependent (waiting in v4, Waiting after)
			wait.ForLog("aiting for connections"),
			wait.ForListeningPort("27017/tcp"),
		),
		Env:      map[string]string{},
		Networks: []string{networkName},
	}

	mongoContainer, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		return nil, err
	}
	mongoHost, err := mongoContainer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	externalEndpoint, err := mongoContainer.Endpoint(ctx, "mongodb")
	if err != nil {
		return nil, err
	}
	internalCS := fmt.Sprintf("mongodb:/%s:27017/horizon", mongoHost.Name)
	externalCS := fmt.Sprintf("%s/horizon", externalEndpoint)
	mongo := MongoContainer{
		Container: mongoContainer,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	configFolder, err := seedConfigFolder(filepath.Join(cwd, "resources", "horizon_conf"), horizonVersion)
	if err != nil {
		return nil, err
	}

	dbPath := filepath.Join(configFolder, "db")
	t.Logf("Loading DB from %s", dbPath)
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return nil, err
	}

	importGroup, _ := errgroup.WithContext(ctx)
	for _, e := range entries {
		if !e.IsDir() {
			entry := e
			importGroup.Go(func() error {
				cmd := exec.CommandContext(
					ctx,
					"mongoimport",
					"--db=horizon",
					"--jsonArray",
					"--file="+filepath.Join(dbPath, entry.Name()),
					externalCS,
				)
				out, err := cmd.CombinedOutput()
				if err != nil {
					return fmt.Errorf("mongoimport %s: %w\n%s", entry.Name(), err, out)
				}
				t.Logf("mongoimport %s: ok", entry.Name())
				return nil
			})
		}
	}
	if err = importGroup.Wait(); err != nil {
		return nil, err
	}

	horizonEnv := make(map[string]string)
	horizonEnv["LICENSE"] = os.Getenv("LICENSE")
	horizonEnv["MONGODB_URI"] = internalCS
	horizonEnv["APPLICATION_SECRET"] = "arandomsecretthatisverylongotherwiseplaywillnotbehappy"
	horizonEnv["EVENT_SEAL_SECRET"] = "arandomsecretthatisverylongotherwiseplaywillnotbehappy"
	horizonEnv["HOSTS_ALLOWED.0"] = "."
	horizonEnv["HTTP_CERTIFICATE_HEADER"] = "SSL_CLIENT_CERT"
	horizonEnv["ACME_URL_SCHEME"] = "http"

	envPath := filepath.Join(configFolder, ".env")
	t.Logf("Loading ENV from %s", envPath)

	envFileContents, err := os.ReadFile(envPath)
	if err != nil {
		return nil, err
	}

	for _, line := range strings.Split(string(envFileContents), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Split on =
		varName, varValue, foundEqual := strings.Cut(line, "=")
		if !foundEqual {
			t.Logf("WARNING: Could not find = in .env file '%s'", envPath)
		}

		horizonEnv[varName] = varValue
	}

	registry := "quay.io/evertrust/"
	if os.Getenv("PREVIEW") != "" {
		registry = ""
	}
	horizonImage := registry + "horizon:" + horizonVersion
	if image := os.Getenv("HRZ_IMAGE"); image != "" {
		horizonImage = image
	}

	// Pre-assign a name to the Horizon container so that Nginx can reference it
	// before Horizon is ready, allowing both to start in parallel.
	horizonContainerName := fmt.Sprintf("horizon-%d", time.Now().UnixNano())

	horizonReq := testcontainers.ContainerRequest{
		Name:         horizonContainerName,
		Image:        horizonImage,
		WaitingFor:   wait.ForLog("GRADING-START").WithStartupTimeout(3 * time.Minute),
		Networks:     []string{networkName},
		ExposedPorts: nil,
		Env:          horizonEnv,
	}
	nginxReq := testcontainers.ContainerRequest{
		Image:    "nginx:1",
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"nginx-horizon"},
		},
		ExposedPorts: []string{"80/tcp", "443/tcp"},
		WaitingFor:   wait.ForLog("ready for start up"),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath: filepath.Join(
					cwd,
					"etc",
					"nginx",
					"templates",
					"nginx.conf.template",
				),
				ContainerFilePath: "/etc/nginx/templates/nginx.conf.template",
			},
			{
				HostFilePath:      filepath.Join(cwd, "etc", "ssl", "chain.pem"),
				ContainerFilePath: "/var/ssl/chain.pem",
			},
			{
				HostFilePath:      filepath.Join(cwd, "etc", "ssl", "key.pem"),
				ContainerFilePath: "/var/ssl/key.pem",
			},
		},
		Env: map[string]string{
			"NGINX_ENVSUBST_OUTPUT_DIR": "/etc/nginx",
			"HORIZON_HOST":              horizonContainerName,
		},
	}

	startGroup, gctx := errgroup.WithContext(ctx)
	var horizonContainer testcontainers.Container
	var nginxContainer testcontainers.Container

	startGroup.Go(func() error {
		var startErr error
		horizonContainer, startErr = testcontainers.GenericContainer(gctx, testcontainers.GenericContainerRequest{
			ContainerRequest: horizonReq,
			Started:          true,
		})
		return startErr
	})
	startGroup.Go(func() error {
		var startErr error
		nginxContainer, startErr = testcontainers.GenericContainer(gctx, testcontainers.GenericContainerRequest{
			ContainerRequest: nginxReq,
			Started:          true,
		})
		return startErr
	})

	if err = startGroup.Wait(); err != nil {
		return nil, err
	}

	horizonC := HorizonContainer{
		Container: horizonContainer,
	}

	http := "http://nginx-horizon"
	if os.Getenv("DOCKER_NETWORK") == "" {
		http, err = nginxContainer.PortEndpoint(ctx, "80", "http")
		if err != nil {
			return nil, err
		}
	}
	nginx := NginxContainer{
		Container: nginxContainer,
		HttpUrl:   http,
	}

	return &HorizonTestInstances{
		Nginx:   &nginx,
		Mongo:   &mongo,
		Horizon: &horizonC,
	}, nil
}
