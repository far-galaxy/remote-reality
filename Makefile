include .env
export

NC=\033[0m
GREEN=\033[1;32m
BLUE=\033[1;34m
CYAN=\033[1;36m

build: 
	@echo "${BLUE}Building...${NC}"
	@go build
	@chmod +x remote_reality
	@echo "${GREEN}Build succesful!${NC}"

build-remote: 
	@echo "${CYAN}Building for remote (Raspberry)...${NC}"
	@env GOOS=linux GOARCH=arm GOARM=7 go build
	@echo "${GREEN}Build succesful!${NC}"

deploy: build-remote
	@scp remote_reality ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_REPO}
	@scp -r site ${REMOTE_USER}@${REMOTE_HOST}:${REMOTE_REPO}
	@ssh ${REMOTE_USER}@${REMOTE_HOST} chmod +x ${REMOTE_REPO}/remote_reality
	@echo "${GREEN}Deploy succesful!${NC}"

run: build
	@./remote_reality ${OPTIONS}

run-remote:
	@ssh ${REMOTE_USER}@${REMOTE_HOST} 'cd ${REMOTE_REPO} && sudo ./remote_reality'

clean:
	@rm remote_reality