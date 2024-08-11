<div align="center">
  <img src="assets/img/icon.png" alt="Magi Icon" height="130"/>
</div>

<div align="center">
  <img alt="GitHub Release" src="https://img.shields.io/github/v/release/alexander-bruun/magi">
  <img alt="GitHub commit activity" src="https://img.shields.io/github/commit-activity/m/alexander-bruun/magi">
  <img alt="GitHub License" src="https://img.shields.io/github/license/alexander-bruun/magi">
  <img alt="GitHub Sponsors" src="https://img.shields.io/github/sponsors/alexander-bruun">
</div>

# Magi

**Magi** is a minimalist and efficient manga indexer, organizer, and reader. It does **NOT** distribute copyrighted material, including posters, metadata, or any other content owned by original copyright holders. Magi is designed solely as a local application to manage your digital manga collection using common file formats like `.cbz`, `.cbr`, `.zip`, `.rar`, etc. Metadata and posters are fetched from publicly accessible APIs to enhance the user experience.

![Magi Frontpage](/docs/images/frontpage.png)

Additional Magi screenshots, can be found under `/docs/images`, we add example page screenshots as new features are added.

> [!TIP]
> Due to the heavy compression of rar files, you will incur performance issues. So it is recommended to use traditional zip files when possible, due to their performance benefits for random reads and writes.

Magi builds to a single binary targeting: `Linux`, `MacOS` and `Windows` on the following architectures: `amd64` and `arm64`. If additional platforms should be supported, then feel free to open a merge request to the pipelines so more people can enjoy Magi.

Binary releases are uploaded to the corresponding [GitHub Release](https://github.com/alexander-bruun/magi/releases) bound to a Git Tag generated through the GitHub workflow pipelines triggered by a merge to main, because of this we primarily work in the `next` branch, and merge to `main` when significant changes has been made for a tag bump to be reasonable.

If you wish to run Magi as a Docker container, then fear not! We build Docker container images for `linux` on `amd64` and `arm64`, which can be found on [Docker Hub](https://hub.docker.com/repository/docker/alexbruun/magi/tags) and GHCR (Coming soon).

When running with native binaries it is heavily recommended to use something like [shawl](https://github.com/mtkennerly/shawl) on Windows to run Magi as a service in the backgounrd, and registering a Unit on Linux.

Alternatively, run Magi in a container solution such as Kubernetes, Docker Desktop or Podman... the sky is the limit! Just make sure the underlying data is made available to the native or container environment.

We can be found on Discord for help, questions or just hanging around.

<a target="_blank" href="https://discord.gg/2VDmSUxGkE"><img src="https://dcbadge.limes.pink/api/server/2VDmSUxGkE" /></a>

> [!NOTE]
> If you are a Discord wizard, reach out, we are looking for help configuring the Discord server.

## Technologies

Magi is built with the following technologies:

- [GoLang](https://go.dev/) - Programming Language
- [Bolt](https://github.com/etcd-io/bbolt) - Key value store
- [GoFiber](https://docs.gofiber.io/) - HTTP Server
- [Templ](https://templ.guide/) - HTML Templating
- JavaScript libraries:
  - [HTMX](https://htmx.org/) - Hypermedia
  - [Lazysizes](https://github.com/aFarkas/lazysizes) - Lazy image loading
- [Tailwind CSS](https://tailwindcss.com/) - CSS Framework
- [Franken UI](https://franken-ui.dev/) - Predefined Components
- [Mangadex API](https://api.mangadex.org/docs/) - Metadata Scraping

Magi is compiled into a single binary file, making it highly portable and easy to run on any machine (meaning there is no "installer" it is by design portable). The build process integrates static views and assets into the final binary, allowing for fast builds and quick testing.

> [!NOTE]
> Mangadex APi was chosen over other solutions due to it allowing anonymous requests and not forcing the end-user to provide API tokens or keys. Alternatives like MAL was explored, and worked just fine, but was a pain for people to indiviually create their own API tokens etc...

## Getting Started

To set up Magi for development, use the following command in the project directory:

```sh
air
```

This will start the application and provide you with logs indicating the status of the server and other components. You can then access the application at `http://localhost:3000`. Air also provides similar functionaly to something like `next run dev` where you get a proxy page that reloads for you, by opening the application on port `:3001` then you will get proxy refresh's when you change the source code.

This provides a smoother developer experience instead of having to refresh the page every time you made a change.

To regenerate the Tailwind CSS theme with a new color scheme, run:

```sh
npx tailwindcss -i ./input.css -o ./assets/css/styles.css --minify
```

> [!NOTE]
> Make sure to update the theme in `tailwind.config.js` before running this command.

If you want to inspect the data stored in the Bolt key-value store, the `bbolt` CLI can be used. Alternatively a community Open-Source project named `boltbrowser` can be used, the project can be found [here](https://github.com/br0xen/boltbrowser).

```sh
go install github.com/br0xen/boltbrowser@latest
boltbrowser ~/magi/magi.db
```

This will open a interactive console browser, here you can explore individual buckets, and the data contained within them.

## Contributing

Magi is in its early stages of development, and many features are still in progress. Contributions are welcome! Please feel free to submit merge requests or feature requests. Your input is invaluable for shaping the direction of Magi.

## License

[MIT License](LICENSE)
