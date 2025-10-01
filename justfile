export PATH := "./node_modules/.bin:" + env_var('PATH')

dev tags='':
    fd 'go|templ|base.css' | entr -r bash -c 'templ generate && go build -tags={{tags}} -o /tmp/njump && TAILWIND_DEBUG=true PORT=3001 /tmp/njump'

build: templ tailwind
    go build -o ./njump

deploy target: templ tailwind
    CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC=$(which musl-gcc) go build -tags='libsecp256k1' -ldflags="-linkmode external -extldflags '-static' -X main.compileTimeTs=$(date '+%s')" -o ./njump
    scp njump {{target}}:njump/njump-new
    ssh {{target}} 'systemctl stop njump'
    ssh {{target}} 'mv njump/njump-new njump/njump'
    ssh {{target}} 'systemctl start njump'

templ:
    templ generate

protobuf:
    protoc --proto_path=. --go_out=. --go_opt=paths=source_relative internal.proto

prettier:
    prettier -w templates/*.html

tailwind:
    tailwind -i base.css -o static/tailwind-bundle.min.css --minify

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
        $(chromium @(base_url + '/image/' + code))
