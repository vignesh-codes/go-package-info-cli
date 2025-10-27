# Package Statistics

A command-line tool that analyzes Debian package statistics by downloading and parsing Contents files from Debian repositories. It outputs the top 10 packages that have the most files associated with them. 

## Example Output

```bash
$ ./build/package_statistics arm64
2025/09/10 00:04:55 Starting download from http://ftp.uk.debian.org/debian/dists/stable/main/Contents-arm64.gz
2025/09/10 00:04:55 Downloading 12554723 bytes (12.0 MB)
[██████████████████████████████████████████████████] 100.00% (12.0/12.0 MB, 3.4 MB/s, ETA: 0s)2025/09/10 00:04:58 Download completed
Rank  Package Name                   Count
--------------------------------------------------
1     devel/piglit                             54424
2     math/acl2-books                          20287
3     science/esys-particle                    18127
4     libdevel/libboost1.88-dev                16001
5     libdevel/libboost1.83-dev                15662
6     lisp/racket                              12202
7     libdevel/libstarpu-dev                   9652
8     libdevel/libtorch-dev                    8672
9     net/zoneminder                           8161
10    kernel/linux-headers-6.12.43+deb13-arm64 7476
```

And when the cache exists
```bash
/canonical$ ./build/package_statistics  arm64
2025/09/10 01:12:00 Using recent cached data (age=29m22s)
Rank  Package Name                   Count
--------------------------------------------------
1     devel/piglit                             54424
2     math/acl2-books                          20287
3     science/esys-particle                    18127
4     libdevel/libboost1.88-dev                16001
5     libdevel/libboost1.83-dev                15662
6     lisp/racket                              12202
7     libdevel/libstarpu-dev                   9652
8     libdevel/libtorch-dev                    8672
9     net/zoneminder                           8161
10    kernel/linux-headers-6.12.43+deb13-arm64 7476
```


## Features
- Caching with ETag/Last-Modified
- Concurrency-safe cache writes
- Retry + timeout for downloads
- Progress bar for large files
- Extensible clean architecture
- Added makefile to perform install, build, test, lint, vet, scan (for vuln), fmt


# Thought Process and Approach
## Thought Process
- My first milestone was to get the data from the mirror endpoint and parse the data to get the top packages with the most files. 
I found it quite straightforward to do them using simple http get request and parsing the data by iterating through the lines and counting the packages.

- I pretty much done with this within like 1 to 2 hours. Then I wanted to implement the caching mechanism to avoid downloading the data again and again.
I remember whenever I want to install a new debian or ubuntu package, it might sometime say not found. Then doing "apt update" will help.
So I did some readings on how to implement it effectively without having to hit the mirror endpoint again and again.

The way I implemented it was using response headers. I found that debian package contents file is not updated frequently. So I can use the ETag and Last-Modified headers to avoid downloading the data again and again.

When I did HEAD method request, I found that it can take in the ETag and Last-Modified headers and respond with 304 if the data is not modified.

I came up with the following logic:
- I maintain the flag force-refresh to avoid using the cached data.
- If the force-refresh is true, the new data will be downloaded and the cache will be updated.
- If the force-refresh is false, it will check if the cache exists
     - if the cache exists, it will check if the data is recent (less than 1 hour)
     - if not, it will hit the HEAD request to check if the data is modified
- When then store the entire data for that architecture in the cache in this format:
```json
{
    "architecture": "arm64",
    "stats": [
    {
        "name": "devel/piglit",
        "file_count": 54424
    },
  ],
  "timestamp": "2025-09-10T02:45:23.444001455Z",
  "etag": "\"68bbfd64-bf91e3\"",
  "last_modified": "Sat, 06 Sep 2025 09:22:44 GMT",
  "url": "http://ftp.uk.debian.org/debian/dists/stable/main/Contents-arm64.gz",
  "checksum": "b21bb08661f8a4c0d7562f88c6608340"
}
```
- The etag and last_modified are the headers from the HEAD request.
- We are not handling checksum and gpg verifications.

One this is done, I was pretty much happy with the results and moved on to improving the code quality, tests, makefile and documentations.

## Code level optimizations and fault-tolerant mechanisms I added

- Lock Mechanism
    - Since we are storing the cache as a json file, it is possible that two processes can write to the file at the same time.
    - So I added multi-arch os friendly lock mechanism using flock.
    - Also added a cleanup mechanism, timeouts (to prevent deadlock), and context cancellation to remove the lock file if it is stale.
    - We update to json only after the data is downloaded and parsed successfully as a one time write/update.

- Error Handling
    - I added a retry mechanism to handle network errors.
    - Added timeouts to handle long downloads.
    - Added a fallback mechanism to use the cached data if the download fails.
    - Added a cleanup mechanism to remove the cache file if it is corrupted.
    - Added a graceful shutdown mechanism to handle user interruption.
    - Bufferio to read the file line by line to avoid reading the entire file into memory at once making it memory efficient.

- Progress Reporting
    - Added a progress reporting mechanism to show the download progress in real-time.



## Usage

### Created Make file to build the binary and run the tests, lint, vet, scan.

```bash
$ make help
Common make targets:
  build         - Build the binary
  run           - Run the binary
  test          - Run tests
  fmt           - Format source code
  vet           - Run go vet
  lint          - Run linters
  deps          - Download dependencies
  clean         - Remove build artifacts
  scan          - Scan dependencies for vulnerabilities
  ci            - Run all CI checks - runs tests, lint, vet, scan
```

Make sure to install the dependencies like golangci-lint, govulncheck
```bash
$ go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
$ go install golang.org/x/vuln/cmd/govulncheck@latest
$ export PATH=$PATH:$(go env GOPATH)/bin
```

Then run the following commands to build the binary and run the tests, lint, vet, scan.

```bash
$ make install
$ make ci
$ make build
```

```bash
# Basic usage - analyze amd64 packages
./build/package_statistics amd64

# Show top 20 packages instead of 10
./build/package_statistics -top 20 amd64

# Force refresh cache
./build/package_statistics -force-refresh amd64

# Set custom download timeout (0 = no timeout)
./build/package_statistics -download-timeout 5m amd64

# Use custom cache directory
./build/package_statistics -cache-dir ~/.my-cache amd64
```

## Command Line Options
```bash
$ ./build/package_statistics -help
Usage of ./build/package_statistics:
  -cache-dir string
        cache directory (default ".cache/package-statistics")
  -cache-ttl duration
        cache TTL (default 24h0m0s)
  -download-timeout duration
        download timeout (0 = no timeout) (default 10m0s)
  -force-refresh
        force refresh cache
  -help
        show help
  -top int
        number of top packages (default 10)
```


## Testing Approach

- I created a test_out.txt file to store the expected output of the tests.
- I created some simple unit tests for the app logic, caching, and progress reporting and managed to achieve more than 70% coverage. (unit tests were created with the help of some gpt generated code)

I mostly did manual testing to ensure the code was working as expected. If I had time, I would add some more scenario based tests. For now, I focussed more on testing by changing the cached etag and last_modified information. Also tested the concurrency scenario by running multiple instances of the program at the same time.

I also did some manual testing on the count of files per package by outputting the raw data and checking the count of some randomly selected.

## Folder Structure

```bash
/canonical$ tree .
.          
├── Makefile
├── README.md
├── build
│   └── package_statistics
├── cmd
│   └── package_statistics
│       └── main.go
├── docs
│   └── notes.txt
├── go.mod
├── go.sum
├── internal
│   ├── app
│   │   ├── app.go
│   │   ├── app_test.go
│   │   ├── download.go
│   │   ├── download_test.go
│   │   ├── utils.go
│   │   └── utils_test.go
│   ├── cache
│   │   ├── cache.go
│   │   └── cache_test.go
│   └── progress
│       ├── progress.go
│       └── progress_test.go
└── test_out.txt

9 directories, 18 files

```

I took inspiration from the clean architecture principles that some of the open source projects I contributed to. 
I created a separate internal package for app (core app logic - download, parsing, utils), caching (for caching related logic), and progress reporting (for output related logic) and used a single go module for entire project (github.com/canonical-dev/package_statistics)

This enables for easier extensions for other packages that might be added in the future (like ubuntu, or outputing in different formats, different cache mechanisms, etc).


## Data Flow

```
1. Input Stage

./package_statistics <architecture>

We load the config from the command line flags or defaults
Config{
    Architecture: "amd64",
    CacheDir: ".cache/package-statistics", 
    CacheTTL: 24h,
    ForceRefresh: false,
    TopCount: 10,
    DownloadTimeout: 10m
}

2. Cache check stage
checks for contents-<arch>.json in the cache directory based on defined logic

3. NETWORK REQUEST STAGE

URL = URL template + architecture
- HEAD Request (with ETag/Last-Modified if cached)
- GET Request (if HEAD indicates new data needed)

4. DOWNLOAD & PROGRESS STAGE

Input: HTTP Response Body (gzip-compressed)
It then decompresses the gzip stream

5. PARSING STAGE
Input: Raw text lines from decompressed file
"usr/bin/file1 pkg1,pkg2,pkg3"
"usr/lib/file2 pkg2,pkg4"
"usr/share/file3 pkg1"

Output: map[string]int
{"pkg1": 1, "pkg2": 1, "pkg3": 1}
{"pkg2": 1, "pkg4": 1}
{"pkg1": 1}

6. SORTING STAGE
Input: map[string]int
Output: []cache.PackageStats

7. CACHE SAVE STAGE
Storing Sorted package statistics + metadata

8. OUTPUT STAGE
PrintTop(sorted stats, top count)

```

## Things I would improve
- More scenario based tests
- Add tests on github CI to test on different architectures and different versions of the program.
- Benchmark testing (for downloading with huge data and low bandwidth, unstable network etc)
