# Add a language to the web app

The web app ships English and German ([ADR 0019](../adr/0019-english-project-language.md)).
This guide adds another language, using French (`fr`) as the example.

**You need:** a checkout of the repository, Go, Node.js (`npm ci` once) — see
[testing](testing.md).

## 1. Copy the English dictionary

```sh
cp internal/web/static/locales/en.json internal/web/static/locales/fr.json
```

The file name is the language code that `navigator.language` starts with
(`fr` matches `fr-FR`, `fr-CH`, …). Every file is a flat JSON object
`"key": "text"`, keys sorted, 2-space indent, UTF-8.

## 2. Translate the values

Change only the values, never the keys.

- Keep placeholders in curly braces exactly as they are, e.g.
  `"np.titleFor": "Now Playing — {name}"` → `"En cours — {name}"`. The order
  inside the sentence may change.
- Plurals are two keys: `key.one` (exactly 1) and `key.other` (everything
  else, including 0). Translate both; `{n}` is the number:

  ```json
  "status.speakers.one": "{n} enceinte",
  "status.speakers.other": "{n} enceintes",
  ```

- Do not translate the product name **Hallido**, "Sonos", or technical tokens
  such as `config.json` or `URL`.
- Use the terms from the [glossary](../reference/glossary.md). Add a column
  for the new language there when you introduce terms.
- No key may be empty. If a text is not translated yet, leave it out of the
  file: the app then shows the English text for that key (and warns once in
  the browser console). The coverage test below still expects the full key
  set before you open a pull request.

## 3. Register the language in `i18n.js`

Open `internal/web/static/i18n.js` and add the code to `LANGUAGES`:

```js
const LANGUAGES = ["en", "de", "fr"];
```

Language detection: the first entry of `LANGUAGES` (other than English) that
`navigator.language` starts with wins; anything else gets English. The app
loads `locales/<code>.json` plus `locales/en.json` as fallback.

Optionally set the locale for dates and sorting in `LOCALES`
(`{ de: "de-AT", fr: "fr-FR" }`); without an entry, `en-US` is used.

Nothing else needs to change: the agent embeds every `locales/*.json`
(`internal/web/web.go`), adds the app version to its URL and the service worker
keeps it offline.

## 4. Run the coverage tests

```sh
go test ./internal/web/          # all locales parse, same keys as en.json, no empty value
node scripts/test/i18n-lint.js   # format (sorted keys), keys, placeholders
make test-gui                    # static checks + Playwright (i18n.spec.js and layout)
```

Long translations can break the layout on a phone. To look at the app in the
new language, start Chromium with `--lang=fr` or set the browser language,
then open the web app (see [local start](local-start.md)). Check at least the
tab bar, the scene editor and the schedule editor at 390 px width.

## 5. Open a pull request

Commit with a Conventional Commit message, for example
`feat(i18n): add French web app texts`, and open a pull request. Mention who
reviewed the wording if you are not a native speaker.
