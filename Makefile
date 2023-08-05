.DEFAULT_GOAL := build_run


run:
	go run main.go ${BRANCH_NAME}


build:
	go build


build_run: build
	./cicli ${BRANCH_NAME}


.PHONY: run build build_run
