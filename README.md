# Load Tester

This CLI tool can be used to load test a REST API endpoint.

## Usage

First, you'll need to build the tool locally or pull the Docker image:

`make build` or `docker pull nrfiacco/loadtest`

Then, you can run the tool via:

`./bin/loadtest [flags] [target]`

or

`docker run -v ./out:/app/out loadtest [flags] [target]`

**Note**: you'll need to mount a local directory as a volume in the Docker container, and specify all paths relative
to that container in your flags:

`docker run -v ./out:/app/out loadtest --output_file out/output.csv https:test-url.com`

### Flags

```
--duration
  Duration of the test in Golang Duration notation. Defaults to 0 (infinity)

--qps
  Queries per second. Defaults to 100

--workers
  Number of workers to use for the test. Defaults to 10

--timeout
  Timeout to wait for each request in seconds. Defaults to 30

--method
  HTTP method to use for requests. Defaults to GET

--output_file
  Output file to write results to. Defaults to \"stdout\"
```

## Building the Docker Image Locally

`docker build -t [your_docker_hub_username]/loadtest .`
