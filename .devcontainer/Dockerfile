FROM fedora:42

COPY . .

ENV PATH "$PATH:/usr/local/go/bin:/home/user/go/bin:/home/user/.cargo/bin"

# install curl, git, ...
RUN dnf update -y
RUN dnf install -y curl git jq go

RUN useradd -m user
USER user

RUN pwd
RUN ls -lhrt

RUN go install github.com/air-verse/air@latest
RUN go install