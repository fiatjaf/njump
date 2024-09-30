export PATH := "./node_modules/.bin:" + env_var('PATH')

dev tags='':
    fd 'go|templ|base.css' | entr -r bash -c 'templ generate && go build -tags={{tags}} -o /tmp/njump && TAILWIND_DEBUG=true PORT=3001 /tmp/njump'

build: templ tailwind
    go build -o ./njump

deploy target: templ tailwind
    CGO_CFLAGS="-I$(pwd)/secp256k1-master/musl/include/" CGO_LDFLAGS="-L$(pwd)/secp256k1-master/musl/lib" GOOS=linux GOARCH=amd64 CC=$(which musl-gcc) go build -tags libsecp256k1 -ldflags="-s -w -linkmode external -extldflags '-static' -X main.compileTimeTs=$(date '+%s')" -o ./njump
    scp njump {{target}}:njump/njump-new
    ssh njump 'systemctl stop njump'
    ssh njump 'mv njump/njump-new njump/njump'
    ssh njump 'systemctl start njump'

libsecp256k1:
    wget https://github.com/bitcoin-core/secp256k1/archive/refs/heads/master.zip
    unzip master.zip
    rm master.zip
    cd secp256k1-master && ./autogen.sh && CC=$(musl-cc) ./configure --enable-module-extrakeys --enable-module-schnorrsig --prefix=$(pwd)/musl && make install

templ:
    templ generate

prettier:
    prettier -w templates/*.html

tailwind:
    tailwind -i base.css -o static/tailwind-bundle.min.css --minify

test:
    go test -tags=nocache

check-samples:
    #!/usr/bin/env xonsh
    base_url = ${...}.get('SERVICE_URL')
    if not base_url:
        output = $(netstat -tulpn 2>&1 | grep njump | awk '{print($4)}')
        port = output.split(':')[-1].strip()
        if not port:
            print('njump not running or could not be found, you can set $SERVICE_URL to specify a base url manually')
            import sys
            sys.exit(4)
        base_url = 'http://localhost:' + port
    else:
        if base_url.endswith('/'):
            base_url = base_url[0:-1]
    samples = $(cat samples.txt).splitlines()
    for code in samples:
        $(chromium @(base_url + '/' + code))
        $(chromium @(base_url + '/njump/image/' + code))
