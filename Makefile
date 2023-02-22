VERSION = 0.0.181

build: 
		go build -o terraform-provider-horizon
		mkdir -p ~/.terraform.d/plugins/evertrust.fr/horizon/horizon/$(VERSION)/darwin_amd64
		cp terraform-provider-horizon ~/.terraform.d/plugins/evertrust.fr/horizon/horizon/$(VERSION)/darwin_amd64

init: 
		terraform init -upgrade

apply:
		terraform apply -auto-approve

update:
		go get github.com/evertrust/horizon-go@latest
		go mod tidy