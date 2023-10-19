build:
    CC=$(which musl-gcc) go build -ldflags='-s -w -linkmode external -extldflags "-static"' -o ./njump

deploy: build
    ssh root@turgot 'systemctl stop njump'
    rsync njump turgot:njump/njump-new
    ssh turgot 'mv njump/njump-new njump/njump'
    ssh root@turgot 'systemctl start njump'

refresh_build:
    sass static/styles.scss static/styles.css
    prettier -w static/*.css static/*.scss templates/*.html
    go build -tags=nocache -o ./tmp/main .
