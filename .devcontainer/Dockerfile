FROM fedora:latest

COPY . .

ENV PATH "$PATH:/usr/local/go/bin:/home/user/go/bin:/home/user/.cargo/bin"

# install curl, git and go
RUN dnf update -y
RUN dnf install -y git go

RUN useradd -m user
USER user

RUN go install github.com/air-verse/air@latest
RUN go install github.com/a-h/templ/cmd/templ@latest

RUN air