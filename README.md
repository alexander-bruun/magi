# Magi

**Magi** is a minimalist and efficient manga indexer, organizer, and reader. It does **not** distribute copyrighted material, including posters, metadata, or any other content owned by original copyright holders. Magi is designed solely as a local application to manage your digital manga collection using common file formats like `.cbz`, `.cbr`, `.rar`, etc. Metadata and posters are fetched from publicly accessible APIs to enhance the user experience.

![Magi Frontpage](/assets/img/frontpage.png)

Additional Magi screenshots, can be found under `/assets/img`, we add example page screenshots as new features are added.

## Technologies

Magi is built with the following technologies:

- [GoLang](https://go.dev/) - Programming Language
- [Gorm](https://gorm.io/index.html) - Database Migrations
- [GoFiber](https://docs.gofiber.io/) - HTTP Server
- [Templ](https://templ.guide/) - HTML Templating
- JavaScript libraries:
  - [HTMX](https://htmx.org/) - Hypermedia
  - [Lazysizes]([https://htmx.org/)](https://github.com/aFarkas/lazysizes) - Lazy image loading
- [Tailwind CSS](https://tailwindcss.com/) - CSS Framework
- [Franken UI](https://franken-ui.dev/) - Predefined Components
- [Mangadex API](https://api.mangadex.org/docs/) - Metadata Scraping

Magi is compiled into a single binary file, making it highly portable and easy to run on any machine. The build process integrates static views and assets into the final binary, allowing for fast builds and quick testing.

> Mangadex APi was chosen over other solutions due to it allowing anonymous requests and not forcing the end-user to provide API tokens or keys. Alternatives like MAL was explored, and worked just fine, but was a pain for people to indiviually create their own API tokens etc...

## Getting Started

To set up Magi for development, use the following command in the project directory:

```sh
$ air
```

This will start the application and provide you with logs indicating the status of the server and other components. You can then access the application at `http://localhost:3000`.

To regenerate the Tailwind CSS theme with a new color scheme, run:

```sh
npx tailwindcss -i ./input.css -o ./assets/css/styles.css --minify
```

Make sure to update the theme in `tailwind.config.js` before running this command.

## Contributing

Magi is in its early stages of development, and many features are still in progress. Contributions are welcome! Please feel free to submit merge requests or feature requests. Your input is invaluable for shaping the direction of Magi.

## License

[MIT License](LICENSE)
