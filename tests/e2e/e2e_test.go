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
	ConnectionString string
}

type HorizonContainer struct {
	testcontainers.Container
	ConnectionUrl string
	Version       string
}

type NginxContainer struct {
	testcontainers.Container
	HttpUrl  string
	HttpsUrl string
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

	logPath := filepath.Join(cwd, "..", "reports", "horizon.log")

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
		Container:        mongoContainer,
		ConnectionString: internalCS,
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	horizonMajor := strings.Join(strings.Split(horizonVersion, ".")[:2], ".")

	configFolder := filepath.Join(cwd, "resources", "horizon_conf", horizonMajor)

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

	// Pre-assign a name to the Horizon container so that Nginx can reference it
	// before Horizon is ready, allowing both to start in parallel.
	horizonContainerName := fmt.Sprintf("horizon-%d", time.Now().UnixNano())

	horizonReq := testcontainers.ContainerRequest{
		Name:         horizonContainerName,
		Image:        registry + "horizon:" + horizonVersion,
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

	horizonInspect, err := horizonContainer.Inspect(ctx)
	if err != nil {
		return nil, err
	}
	horizonC := HorizonContainer{
		Container:     horizonContainer,
		Version:       horizonVersion,
		ConnectionUrl: fmt.Sprintf("%s:9000", horizonInspect.Name),
	}

	http := "http://nginx-horizon"
	https := "https://nginx-horizon"
	if os.Getenv("DOCKER_NETWORK") == "" {
		http, err = nginxContainer.PortEndpoint(ctx, "80", "http")
		if err != nil {
			return nil, err
		}
		https, err = nginxContainer.PortEndpoint(ctx, "443", "https")
		if err != nil {
			return nil, err
		}
	}
	nginx := NginxContainer{
		Container: nginxContainer,
		HttpUrl:   http,
		HttpsUrl:  https,
	}

	return &HorizonTestInstances{
		Nginx:   &nginx,
		Mongo:   &mongo,
		Horizon: &horizonC,
	}, nil
}
