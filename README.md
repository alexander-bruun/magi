# Magi

Magi is a simplistic and performant manga indexer, organizer and reader. Magi does **NOT** distribute copyrighted material and / or posters, metadata or any other material that belongs to the original copyright holders. Magi is strictly a local application, used to index a digital manga collection using common digital comic file formats, such as: .cbz, .cbr, .rar etc...

![Magi Frontpage](/assets/img/frontpage.png)

Magi is built with the following technologies:

- [GoLang](https://go.dev/) (language)
- [Gorm](https://gorm.io/index.html) (database migrations)
- [GoFiber](https://docs.gofiber.io/) (http server)
- [Templ](https://templ.guide/) (html templating)
- [HTMX](https://htmx.org/) (Hypermedia)
- [Tailwind CSS](https://tailwindcss.com/) (CSS framework)
- [Franken UI](https://franken-ui.dev/) (Predefined components)
- [Mangadex API](https://api.mangadex.org/docs/) (Metadata scraping)

Since everything is contained within this GoLang project, it compiles to a single tiny binary. Builds for the most common platforms can be generated with `./build-release.sh`, this will embed static views and assets into the final binary.

This approach enables fast builds, and even faster testing. And allows practically any machine, anywhere to run Magi with a singular binary! What is there not to like?

> Magi is in **very** early stages, so entire parts may be missing and /or incomplete. Feel free to contribute with merge-requests, or feature requests in the issues, without user input Magi will strictly be designed around my own requirements.

## Development

To get Magi up and running for development, simply run the following command in the project directory.

```sh
$ air

building...
(✓) Complete [ updates=8 duration=21.128101ms ]
running...
2024/07/19 20:54:50.987683 main.go:71: [Info] Using '/home/alexander-bruun/magi/magi.db' as the database location and '/home/alexander-bruun/magi/cache' as the image caching location.
2024/07/19 20:54:50.991600 routes.go:11: [Info] Initializing GoFiber routes!
2024/07/19 20:54:50.991672 routes.go:45: [Info] /home/alexander-bruun/magi/cache
2024/07/19 20:54:50.991797 scheduler.go:31: [Info] Initializing manga indexer!
2024/07/19 20:54:50.991833 scheduler.go:60: [Info] Library indexer: 'My Manga Library' has been registered (1 * * * *)!

 ┌───────────────────────────────────────────────────┐
 │                Magi v0.0.1 (alpha)                │
 │                   Fiber v2.52.5                   │
 │               http://127.0.0.1:3000               │
 │       (bound on host 0.0.0.0 and port 3000)       │
 │                                                   │
 │ Handlers ............ 41  Processes ........... 1 │
 │ Prefork ....... Disabled  PID ............. 39656 │
 └───────────────────────────────────────────────────┘
```

Then go to `localhost:3000`, profit!

If you wish to re-generate the Tailwind CSS theme with a different color, run `npx tailwindcss -i ./input.css -o ./assets/css/styles.css --minify` after changing the theme in `tailwind.config.js`.

## Quick commands for testing

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

curl -X PUT \
  http://localhost:3000/api/libraries \
  -H 'Content-Type: application/json' \
  -d '{
  "ID": 1,
  "Name": "My Manga Library",
  "Description": "A library for all my manga!",
  "Cron": "0 * * * *"
}'

curl -X DELETE http://localhost:3000/api/libraries/1
```

## TO-DO:

### Local fallback metadata scraping (cover art extraction, .cbz metadata file?)

### Create a scalable logo for Magi, and add it as the favicon and maybe in the navbar?

### Fix navbar dropdowns disappearing under images / elements

### API middleware and cors rules

### Swagger API docs

### Update library page to show mangas within the library

### Better index api support? Is there better API's for scraping manga information?

### Chapter cover downloading

https://api.mangadex.org/docs/swagger.html#/

curl -X 'GET' \
 'https://api.mangadex.org/cover?limit=100&manga%5B%5D=d8a959f7-648e-4c8d-8f23-f1f3f8e129f3&order%5BcreatedAt%5D=asc&order%5BupdatedAt%5D=asc&order%5Bvolume%5D=asc' \
 -H 'accept: application/json'

### Add a minimal and tasteful footer to the layout

### Add more content to the home page, maybe a hero?

### Add a view more mangas to the home page next to "Recently added" go to /mangas

### Create a new library

### Edit a existing library

### Delete mangas related to the library when deleted

### Delete chapters related to the manga when deleted

### Force re-scan of library, when force is used metadata is overwritten

### Normal re-scan, primarly used to find new chapters in library outside the normal cron schedule

### All mangas page with pagination and sorting by: title, tag, content rating, year etc...

### Manga page chapters list

### Frontend Manga reader

### Backend Manga reader api

### Makefile creation: test, build, release etc...

### Github pipelines / docker build

### Setup as a service guide: systemd, windows?

### GitHub mkdocs material

### Edit manga metadata after a scan, maybe it got mis-identified?

### Edit chapter metadata after a scan, maybe it got mis-identified?
