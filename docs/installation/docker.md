# Installation Guide for Docker

1. **Pull the Docker Image**

    ```bash
    docker pull alex-bruun/magi:latest
    ```

2. **Run the Docker Container**

    ```bash
    docker run -d --name magi alex-bruun/magi:latest
    ```

3. **Verify Installation**

    Check that the container is running:

    ```bash
    docker ps
    ```

    You should see `magi` listed.
