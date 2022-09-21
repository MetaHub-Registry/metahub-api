DSN="host=localhost port=5432 user=postgres password=password dbname=metahub sslmode=disable timezone=UTC connect_timeout=5"
BINARY_NAME=mh-api
ENV=dev

## build: Build binary
build:
	@echo "Building back end..."
	go build -o ${BINARY_NAME}
	@echo "Binary built!"

## run: builds and runs the application
run: build
	@echo "Starting back end..."
	@env DSN=${DSN} ENV=${ENV} ./${BINARY_NAME} --db-pass=password &
	@echo "Back end started!"

## clean: runs go clean and deletes binaries
clean:
	@echo "Cleaning..."
	@go clean
	@rm ${BINARY_NAME}
	@echo "Cleaned!"

## start: an alias to run
start: run

## stop: stops the running application
stop:
	@echo "Stopping back end..."
	@-pkill -SIGTERM -f "./${BINARY_NAME}"
	@echo "Stopped back end!"

## restart: stops and starts the running application
restart: stop start