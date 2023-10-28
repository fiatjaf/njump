export PATH := "./node_modules/.bin:" + env_var('PATH')

dev:
    TAILWIND_DEBUG=true go run .

build: tailwind
    CC=$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./njump

deploy: build
    ssh root@turgot 'systemctl stop njump'
    rsync njump turgot:njump/njump-new
    ssh turgot 'mv njump/njump-new njump/njump'
    ssh root@turgot 'systemctl start njump'

debug-build: tailwind
    go build -tags=nocache -o ./tmp/main .

prettier:
    prettier -w templates/*.html

tailwind:
    tailwind -i tailwind.css -o static/tailwind-bundle.min.css --minify
