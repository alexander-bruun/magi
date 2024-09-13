# Installation Guide for Linux

## Using Systemd

1. **Download the Binary**

    Download the appropriate binary from the [releases page](https://github.com/alexander-bruun/magi/releases).

2. **Install the Binary**

    Copy the binary to `/usr/local/bin`:

    ```bash
    sudo cp magi /usr/local/bin/
    sudo chmod +x /usr/local/bin/magi
    ```

3. **Create a Systemd Service File**

    Create a file `/etc/systemd/system/magi.service` with the following content:

    ```ini
    [Unit]
    Description=Magi Service

    [Service]
    ExecStart=/usr/local/bin/magi
    Restart=always
    User=nobody

    [Install]
    WantedBy=multi-user.target
    ```

4. **Start and Enable the Service**

    ```bash
    sudo systemctl daemon-reload
    sudo systemctl start magi
    sudo systemctl enable magi
    ```

5. **Verify Installation**

    Check the status of the service:

    ```bash
    sudo systemctl status magi
    ```
