BUILD_DATE = `date +%FT%T%z`
PACKAGE = github.com/domudall/gcu-check
GOEXE ?= go

vendor: ## Install gcu-check dependencies
	glide install

install: vendor ## Install gcu-check binary
	${GOEXE} install ${PACKAGE}
