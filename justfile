export PATH := "./node_modules/.bin:" + env_var('PATH')

dev:
    TAILWIND_DEBUG=true go run .

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

build: tailwind
    go build -o ./njump

deploy: tailwind
    sed -i.bak "s#/tailwind-bundle.min.css#/tailwind-bundle.min.css?$(date +'%Y%m%d%H%M')#g" templates/head_common.html
    GOOS=linux GOARCH=amd64 go build -o ./njump
    mv -f templates/head_common.html.bak templates/head_common.html
    rsync --progress njump njump:njump/njump-new
    ssh njump 'systemctl stop njump'
    ssh njump 'mv njump/njump-new njump/njump'
    ssh njump 'systemctl start njump'

debug-build: tailwind
    go build -tags=nocache -o ./tmp/main .

prettier:
    prettier -w templates/*.html

tailwind:
    tailwind -i tailwind.css -o static/tailwind-bundle.min.css --minify

test:
    go test -tags=nocache
