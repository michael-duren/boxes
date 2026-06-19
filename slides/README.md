# boxes — conference slides

Markdown slide deck for the `boxes` talk, built with [Slidev](https://sli.dev).
All content lives in [`slides.md`](./slides.md) — edit that one file.

## Develop

```sh
cd slides
npm install        # first time only
npm run dev        # opens http://localhost:3030 with live reload
```

Presenter mode (notes + timer): press `o` for overview, or open
<http://localhost:3030/presenter>.

## Build a static site

```sh
npm run build      # outputs static HTML to ./dist (base path /boxes/)
```

Serve `dist/` anywhere. For **GitHub Pages** publishing under
`https://<user>.github.io/boxes/`, the `--base /boxes/` in the build script
already matches that path. Host it at the domain root instead? drop the `--base`
flag in `package.json`.

## Export to PDF / PNG

```sh
npm run export                 # slides.pdf  (needs playwright-chromium)
npm run export -- --format png # one PNG per slide
```

The exporter will prompt to install `playwright-chromium` the first time.

## Editing tips

- Slides are separated by `---` on its own line.
- `<v-clicks>` / `v-click` reveal content step by step.
- Code fences support line highlighting: ```` ```go {2,4} ````  or `{all|1|2-3}`
  for click-through steps.
- Per-slide presenter notes go in an HTML comment (`<!-- ... -->`) at the
  bottom of the slide.

Full syntax: <https://sli.dev/guide/syntax>.
