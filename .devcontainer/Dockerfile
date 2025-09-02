FROM fedora:latest

COPY . .

ENV PATH "$PATH:/usr/local/go/bin:/home/user/go/bin:/home/user/.cargo/bin"

# install curl, git, go and nodejs
RUN dnf update -y
RUN dnf install -y git go nodejs

# install npx globally
RUN npm i -g npx

RUN useradd -m user
USER user

RUN go install github.com/air-verse/air@latest
RUN go install github.com/a-h/templ/cmd/templ@latest