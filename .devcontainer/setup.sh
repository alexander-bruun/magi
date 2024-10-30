# install curl, git, ...
dnf update -y
dnf install -y curl git jq go

useradd -m user
su user

INSTALLED_GO_VERSION=$(go version)
echo "Go version ${INSTALLED_GO_VERSION} is installed"

go install
go install github.com/air-verse/air@latest
go install github.com/a-h/templ/cmd/templ@latest