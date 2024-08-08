BINARY_NAME = online_sandbox

help: ## This help dialog.
	@grep -F -h "##" $(MAKEFILE_LIST) | grep -F -v fgrep | sed -e 's/\\$$//' | sed -e 's/##//'

build:  ## Build binary.
	CGO_ENABLED=0 GOOS=linux go build -o ./bin/${BINARY_NAME} .

dev:  ## Development server with live reload.
	air

clean-packages: ## Clean packages.
	go clean -modcache
