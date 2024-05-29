export PATH := "./node_modules/.bin:" + env_var('PATH')

dev:
    fd --no-ignore-vcs 'go|templ|base.css' | entr -r bash -c 'TAILWIND_DEBUG=true SKIP_LANGUAGE_MODEL=true && templ generate && go build -o /tmp/njump && /tmp/njump'

build: templ tailwind
    go build -o ./njump

deploy: templ tailwind
    GOOS=linux GOARCH=amd64 go build -ldflags="-X main.compileTimeTs=$(date '+%s')" -o ./njump
    rsync --progress njump njump:njump/njump-new
    ssh njump 'systemctl stop njump'
    ssh njump 'mv njump/njump-new njump/njump'
    ssh njump 'systemctl start njump'

debug-build: templ tailwind
    go build -o /tmp/njump .

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
