var __index = {"config":{"lang":["en"],"separator":"[\\s\\-]+","pipeline":["stopWordFilter"]},"docs":[{"location":"index.html","title":"Home","text":""},{"location":"index.html#magi","title":"Magi","text":"<p>Magi is a minimalist and efficient manga indexer, organizer, and reader. It does NOT distribute copyrighted material, including posters, metadata, or any other content owned by original copyright holders. Magi is designed solely as a local application to manage your digital manga collection using common file formats like <code>.cbz</code>, <code>.cbr</code>, <code>.zip</code>, <code>.rar</code>, etc. Metadata and posters are fetched from publicly accessible APIs to enhance the user experience.</p> <p></p> <p>Additional Magi screenshots, can be found under <code>/docs/images</code>, we add example page screenshots as new features are added.</p> <p>Due to the heavy compression of rar files, you will incur performance issues. So it is recommended to use traditional zip files when possible, due to their performance benefits for random reads and writes.</p> <p>Magi builds to a single binary targeting: <code>Linux</code>, <code>MacOS</code> and <code>Windows</code> on the following architectures: <code>amd64</code> and <code>arm64</code>. If additional platforms should be supported, then feel free to open a merge request to the pipelines so more people can enjoy Magi.</p> <p>Binary releases are uploaded to the corresponding GitHub Release bound to a Git Tag generated through the GitHub workflow pipelines triggered by a merge to main, because of this we primarily work in the <code>next</code> branch, and merge to <code>main</code> when significant changes has been made for a tag bump to be reasonable. Due to Magi being in its early stages we also push unsafe directly to <code>main</code> branch, this is due to the project being in a early development stage where we have not yet determined everything.</p> <p>If you wish to run Magi as a Docker container, then fear not! We build Docker container images for <code>linux</code> on <code>amd64</code> and <code>arm64</code>, which can be found on Docker Hub and GHCR (Coming soon).</p> <p>When running with native binaries it is heavily recommended to use something like shawl on Windows to run Magi as a service in the backgounrd, and registering a Unit on Linux.</p> <p>Alternatively, run Magi in a container solution such as Kubernetes, Docker Desktop or Podman... the sky is the limit! Just make sure the underlying data is made available to the native or container environment.</p> <p>We can be found on Discord for help, questions or just hanging around.</p> <p></p> <p>The full documentation, can be found here.</p>"},{"location":"index.html#technologies","title":"Technologies","text":"<p>Magi is built with the following technologies:</p> <ul> <li>GoLang - Programming Language</li> <li>Bolt - Key value store</li> <li>GoFiber - HTTP Server</li> <li>Templ - HTML Templating</li> <li>JavaScript libraries:<ul> <li>HTMX - Hypermedia</li> <li>Lazysizes - Lazy image loading</li> </ul> </li> <li>Tailwind CSS - CSS Framework</li> <li>Franken UI - Predefined Components</li> <li>Mangadex API - Metadata Scraping</li> </ul> <p>Magi is compiled into a single binary file, making it highly portable and easy to run on any machine (meaning there is no \"installer\" it is by design portable). The build process integrates static views and assets into the final binary, allowing for fast builds and quick testing.</p> <p>Mangadex APi was chosen over other solutions due to it allowing anonymous requests and not forcing the end-user to provide API tokens or keys. Alternatives like MAL was explored, and worked just fine, but was a pain for people to indiviually create their own API tokens etc...</p>"},{"location":"index.html#getting-started","title":"Getting Started","text":"<p>To set up Magi for development, use the following command in the project directory:</p> <pre><code>air\n</code></pre> <p>This will start the application and provide you with logs indicating the status of the server and other components. You can then access the application at <code>http://localhost:3000</code>. Air also provides similar functionaly to something like <code>next run dev</code> where you get a proxy page that reloads for you, by opening the application on port <code>:3001</code> then you will get proxy refresh's when you change the source code.</p> <p>This provides a smoother developer experience instead of having to refresh the page every time you made a change.</p> <p>To regenerate the Tailwind CSS theme with a new color scheme, run:</p> <pre><code>npx tailwindcss -i ./input.css -o ./assets/css/styles.css --minify\n</code></pre> <p>Make sure to update the theme in <code>tailwind.config.js</code> before running this command.</p> <p>If you want to inspect the data stored in the Bolt key-value store, the <code>bbolt</code> CLI can be used. Alternatively a community Open-Source project named <code>boltbrowser</code> can be used, the project can be found here.</p> <pre><code>go install github.com/br0xen/boltbrowser@latest\nboltbrowser ~/magi/magi.db\n</code></pre> <p>This will open a interactive console browser, here you can explore individual buckets, and the data contained within them.</p>"},{"location":"installation/docker.html","title":"Installation Guide for Docker","text":"<ol> <li> <p>Pull the Docker Image</p> <pre><code>docker pull alex-bruun/magi:latest\n</code></pre> </li> <li> <p>Run the Docker Container</p> <pre><code>docker run -d --name magi alex-bruun/magi:latest\n</code></pre> </li> <li> <p>Verify Installation</p> <p>Check that the container is running:</p> <pre><code>docker ps\n</code></pre> <p>You should see <code>magi</code> listed.</p> </li> </ol>"},{"location":"installation/kubernetes.html","title":"Kubernetes Deployment Guide","text":""},{"location":"installation/kubernetes.html#prerequisites","title":"Prerequisites","text":"<ul> <li>A Kubernetes cluster up and running.</li> <li><code>kubectl</code> configured to interact with your cluster.</li> </ul>"},{"location":"installation/kubernetes.html#installation-methods","title":"Installation Methods","text":""},{"location":"installation/kubernetes.html#method-1-using-a-persistent-volume-claim","title":"Method 1: Using a Persistent Volume Claim","text":"<ol> <li> <p>Create a Deployment Manifest</p> <p>Save the following YAML as <code>magi-deployment.yaml</code>:</p> <pre><code>apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: magi-deployment\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: magi\n  template:\n    metadata:\n      labels:\n        app: magi\n    spec:\n      containers:\n        - name: magi\n          image: alex-bruun/magi:latest\n          volumeMounts:\n            - mountPath: /data\n              name: magi-data\n      volumes:\n        - name: magi-data\n          persistentVolumeClaim:\n          claimName: magi-pvc\n</code></pre> </li> <li> <p>Create a Persistent Volume Claim</p> <p>Save the following YAML as <code>magi-pvc.yaml</code>:</p> <pre><code>apiVersion: v1\nkind: PersistentVolumeClaim\nmetadata:\n  name: magi-pvc\nspec:\n  accessModes:\n    - ReadWriteOnce\n  resources:\n  requests:\n    storage: 1Gi\n</code></pre> </li> <li> <p>Apply the Manifests</p> <p>Apply the Persistent Volume Claim and the Deployment manifest:</p> <pre><code>kubectl apply -f magi-pvc.yaml\nkubectl apply -f magi-deployment.yaml\n</code></pre> </li> <li> <p>Verify Deployment</p> <p>Check the status of the deployment:</p> <pre><code>kubectl get pods\n</code></pre> <p>Ensure that the pods are running and the volume is mounted correctly.</p> </li> </ol>"},{"location":"installation/kubernetes.html#method-2-mounting-a-host-path","title":"Method 2: Mounting a Host Path","text":"<ol> <li> <p>Create a Deployment Manifest</p> <p>Save the following YAML as <code>magi-deployment-hostpath.yaml</code>:</p> <pre><code>apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: magi-deployment\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: magi\n  template:\n    metadata:\n      labels:\n        app: magi\n    spec:\n      containers:\n        - name: magi\n          image: alex-bruun/magi:latest\n          volumeMounts:\n          - mountPath: /data\n            name: magi-data\n      volumes:\n        - name: magi-data\n          hostPath:\n          path: /path/on/host\n          type: Directory\n</code></pre> <p>Replace <code>/path/on/host</code> with the actual path on the host machine that you want to mount inside the container.</p> </li> <li> <p>Apply the Manifest</p> <p>Apply the deployment manifest:</p> <pre><code>kubectl apply -f magi-deployment-hostpath.yaml\n</code></pre> </li> <li> <p>Verify Deployment</p> <p>Check the status of the deployment:</p> <pre><code>kubectl get pods\n</code></pre> <p>Ensure that the pods are running and the host path is mounted correctly.</p> </li> </ol>"},{"location":"installation/linux.html","title":"Installation Guide for Linux","text":""},{"location":"installation/linux.html#using-systemd","title":"Using Systemd","text":"<ol> <li> <p>Download the Binary</p> <p>Download the appropriate binary from the releases page.</p> </li> <li> <p>Install the Binary</p> <p>Copy the binary to <code>/usr/local/bin</code>:</p> <pre><code>sudo cp magi /usr/local/bin/\nsudo chmod +x /usr/local/bin/magi\n</code></pre> </li> <li> <p>Create a Systemd Service File</p> <p>Create a file <code>/etc/systemd/system/magi.service</code> with the following content:</p> <pre><code>[Unit]\nDescription=Magi Service\n\n[Service]\nExecStart=/usr/local/bin/magi\nRestart=always\nUser=nobody\n\n[Install]\nWantedBy=multi-user.target\n</code></pre> </li> <li> <p>Start and Enable the Service</p> <pre><code>sudo systemctl daemon-reload\nsudo systemctl start magi\nsudo systemctl enable magi\n</code></pre> </li> <li> <p>Verify Installation</p> <p>Check the status of the service:</p> <pre><code>sudo systemctl status magi\n</code></pre> </li> </ol>"},{"location":"installation/podman.html","title":"Installation Guide for Podman","text":"<ol> <li> <p>Pull the Podman Image</p> <pre><code>podman pull alex-bruun/magi:latest\n</code></pre> </li> <li> <p>Run the Podman Container</p> <pre><code>podman run -d --name magi alex-bruun/magi:latest\n</code></pre> </li> <li> <p>Verify Installation</p> <p>Check that the container is running:</p> <pre><code>podman ps\n</code></pre> <p>You should see <code>magi</code> listed.</p> </li> </ol>"},{"location":"installation/windows.html","title":"Installation Guide for Windows","text":""},{"location":"installation/windows.html#prerequisites","title":"Prerequisites","text":"<ul> <li>Ensure you have Shawl installed.</li> </ul>"},{"location":"installation/windows.html#steps","title":"Steps","text":"<ol> <li> <p>Download Shawl</p> <p>Download the Shawl installer from the official releases page.</p> </li> <li> <p>Install Shawl</p> <p>Run the installer and follow the instructions. This will install Shawl and add it to your system's PATH.</p> </li> <li> <p>Install Magi</p> <p>Open a command prompt and execute:</p> <pre><code>shawl install magi\n</code></pre> </li> <li> <p>Verify Installation</p> <p>Check that Magi is installed correctly:</p> <pre><code>magi --version\n</code></pre> <p>You should see the version information displayed.</p> </li> </ol>"},{"location":"usage/configuration.html","title":"Magi configuration guide","text":"<p>More to come :)</p>"},{"location":"usage/getting_started.html","title":"Getting up and running with Magi","text":"<p>More to come :)</p>"},{"location":"usage/troubleshooting.html","title":"Troubleshooting and debugging Magi","text":"<p>More to come :)</p>"}]}