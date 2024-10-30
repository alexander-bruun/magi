FROM fedora:42

COPY . .

ENV PATH "$PATH:/usr/local/go/bin:/home/user/go/bin:/home/user/.cargo/bin"

# install curl, git, ...
RUN dnf update -y
RUN dnf install -y curl git jq go

USER user

RUN go install