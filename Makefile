deploy: github remote_test remote_linux ansible check_output
	true

ansible:
	ansible-playbook --ask-sudo-pass -i deploy/inventory.ini deploy/playbook.yml

remote_linux:
	ssh cod 'cd go && GOPATH=~/go make linux'
	scp cod:go/release/goslow_linux_amd64.zip $(GOPATH)/release
	unzip $(GOPATH)/release/goslow_linux_amd64.zip
	mv goslow_linux_amd64 $(GOPATH)/release/goslow

github:
	ssh cod 'cd projects/goslow && git pull github master'

linux:
	~/software/go/bin/go build -o bin/goslow_linux_amd64 github.com/alexandershov/goslow/
	zip -j release/goslow_linux_amd64.zip bin/goslow_linux_amd64

remote_test:
	ssh cod 'cd go && GOPATH=~/go ~/software/go/bin/go test github.com/alexandershov/goslow/'

check_output:
	test "$$(curl 302.goslow.link)" = '{"goslow": "response"}'

test:
	go test github.com/alexandershov/goslow/

windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 ~/software/go-windows-amd64/bin/go build -o bin/goslow_windows_amd64.exe \
		github.com/alexandershov/goslow
	zip -j release/goslow_windows_amd64.exe.zip bin/goslow_windows_amd64.exe


all: remote_linux remote_windows darwin
	true

remote_windows: github
	ssh cod 'cd go && GOPATH=~/go make windows'
	scp cod:go/release/goslow_windows_amd64.exe.zip $(GOPATH)/release/

darwin:
	go build -o $(GOPATH)/bin/goslow_darwin_amd64 github.com/alexandershov/goslow
	zip -j $(GOPATH)/release/goslow_darwin_amd64 $(GOPATH)/bin/goslow_darwin_amd64
