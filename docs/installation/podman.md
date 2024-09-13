# Installation Guide for Podman

1. **Pull the Podman Image**

    ```bash
    podman pull alex-bruun/magi:latest
    ```

2. **Run the Podman Container**

    ```bash
    podman run -d --name magi alex-bruun/magi:latest
    ```

3. **Verify Installation**

    Check that the container is running:

    ```bash
    podman ps
    ```

    You should see `magi` listed.
