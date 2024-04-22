NAME					:= kubectl-download
DIST					:= ./dist
NEXT_VERSION	:= $(shell semtag final -o)

dep:
	go install mvdan.cc/gofumpt@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

build: 
	go build -o $(DIST)/$(NAME) ./

clean:
	rm -rf $(DIST)

fmt:
	go fmt ./...
	gofumpt -l -w .
	go vet ./...

test: proto
	go test -v

lint:
	golangci-lint run -v

cover:
	go test -coverprofile coverage.out

coverweb: cover
	go tool cover -html=coverage.out

check: fmt lint cover

install: build
	mkdir -p ~/bin
	mv $(DIST)/$(NAME) ~/bin/$(NAME) 

uninstall:
	rm ~/bin/$(NAME)

release:
	@git tag $(NEXT_VERSION)
	echo "pushing to origin"
	@git push origin main
	@git push origin $(NEXT_VERSION)