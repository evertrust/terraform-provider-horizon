FROM golang:1.25-bookworm

# mongoimport (mongodb-org-tools) + curl/unzip for terraform download.
RUN apt-get update \
 && apt-get install -y --no-install-recommends bash gnupg curl ca-certificates unzip \
 && curl -fsSL https://www.mongodb.org/static/pgp/server-7.0.asc | gpg -o /usr/share/keyrings/mongodb-server-7.0.gpg --dearmor \
 && echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/7.0 multiverse" > /etc/apt/sources.list.d/mongodb-org-7.0.list \
 && apt-get update \
 && apt-get install -y --no-install-recommends mongodb-org-tools \
 && rm -rf /var/lib/apt/lists/*

ARG TF_VERSION=1.14.8
RUN ARCH=$(dpkg --print-architecture) \
 && curl -fsSLO https://releases.hashicorp.com/terraform/${TF_VERSION}/terraform_${TF_VERSION}_linux_${ARCH}.zip \
 && unzip terraform_${TF_VERSION}_linux_${ARCH}.zip -d /usr/local/bin \
 && rm terraform_${TF_VERSION}_linux_${ARCH}.zip

# gotestsum: runs go test, streams pretty output, writes JUnit + JSON.
# go-test-report: converts the JSON stream to a standalone HTML report.
RUN go install gotest.tools/gotestsum@latest \
 && go install github.com/evertrust/go-test-report@v1.0.1

WORKDIR /workspace
