# Magi

**Magi** is a minimalist and efficient manga indexer, organizer, and reader. It does **not** distribute copyrighted material, including posters, metadata, or any other content owned by original copyright holders. Magi is designed solely as a local application to manage your digital manga collection using common file formats like `.cbz`, `.cbr`, `.rar`, etc. Metadata and posters are fetched from publicly accessible APIs to enhance the user experience.

![Magi Frontpage](/assets/img/frontpage.png)

## Technologies

Magi is built with the following technologies:

- [GoLang](https://go.dev/) - Programming Language
- [Gorm](https://gorm.io/index.html) - Database Migrations
- [GoFiber](https://docs.gofiber.io/) - HTTP Server
- [Templ](https://templ.guide/) - HTML Templating
- [HTMX](https://htmx.org/) - Hypermedia
- [Tailwind CSS](https://tailwindcss.com/) - CSS Framework
- [Franken UI](https://franken-ui.dev/) - Predefined Components
- [Mangadex API](https://api.mangadex.org/docs/) - Metadata Scraping

Magi is compiled into a single binary file, making it highly portable and easy to run on any machine. The build process integrates static views and assets into the final binary, allowing for fast builds and quick testing.

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

## API Quick Commands

You can interact with the Magi API using the following `curl` commands:

- **Create a Library:**

    ```sh
    curl -X POST \
      http://localhost:3000/api/libraries \
      -H 'Content-Type: application/json' \
      -d '{
        "name": "My Manga Library",
        "cron": "*/1 * * * *",
        "description": "A library for all my manga!",
        "folders": [{
          "Name": "/mnt/e/Manga"
        }]
    }'
    ```

- **Update a Library:**

    ```sh
    curl -X PUT \
      http://localhost:3000/api/libraries \
      -H 'Content-Type: application/json' \
      -d '{
      "ID": 1,
      "Name": "My Manga Library",
      "Description": "A library for all my manga!",
      "Cron": "0 * * * *"
    }'
    ```

- **Delete a Library:**

    ```sh
    curl -X DELETE http://localhost:3000/api/libraries/1
    ```

## Roadmap

Here are some of the planned features and improvements for Magi:

- Implement local fallback metadata scraping (e.g., cover art extraction, `.cbz` metadata files).
- Develop a scalable logo for Magi and integrate it as a favicon and in the navbar.
- Resolve issues with navbar dropdowns disappearing under other elements.
- Add API middleware and configure CORS rules.
- Implement Swagger API documentation.
- Update library page to display manga within the library.
- Explore better APIs for manga information scraping.
- Enable chapter cover downloading.
- Add a minimal, tasteful footer to the layout.
- Enhance the home page with additional content and features.
- Create, edit, and delete manga libraries with comprehensive functionality.
- Implement features for managing manga and chapter metadata post-scan.
- Develop both frontend and backend manga readers.
- Create a Makefile for testing, building, and releasing.
- Set up GitHub pipelines and Docker build.
- Provide setup guides for different environments (e.g., systemd, Windows).
- Improve documentation with GitHub MkDocs Material.

## Contributing

Magi is in its early stages of development, and many features are still in progress. Contributions are welcome! Please feel free to submit merge requests or feature requests. Your input is invaluable for shaping the direction of Magi.

## License

[MIT License](LICENSE)
