export PATH := "./node_modules/.bin:" + env_var('PATH')

dev:
    TAILWIND_DEBUG=true go run .

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
