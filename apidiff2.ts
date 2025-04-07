/*
deno run -A --watch apidiff2.ts
http://localhost:62185/
*/

const testPaths = [
  "/v1/full/users?id=0EoAm&user_id=aNzoj",
  "/v1/full/users?id=0EoAm&id=7eP5n&id=aWvp71&id=Wem1e&user_id=aNzoj",

  "/v1/full/users?id=invalid_id1&id=aNzoj",

  "/v1/full/tracks?id=ZaXp01y&user_id=aNzoj",
  "/v1/full/tracks?id=ZaXp01y&id=5zdbywJ&id=Pb4MbAX&id=YVog04p&id=7Mv7ZMP&id=g5yzMy2&id=P7dlqNR&id=RR7krAG&id=O7OdxYV&id=72oXyvr&id=jYWyGG0&id=W6KOm8k&id=Xj554dq&id=gbOmq7k&id=G0z4Ap7&id=Yo0XYMm&id=80jlRwP&id=X63z9Wq&id=lZYWRKb&id=JEz6V0o&id=JZq6550&id=b9rbNdM&id=yy71xr7&id=JGNob5Z&id=VpxQ2Aa&id=qGbEZdz&id=B94qO0j&id=oRGgBka&id=jzwlVj0&id=Y5PNmOr&id=5zRm7dx&id=r3KjOg&id=WVP8qav&id=Nvayj1Y&id=V6GZJ3b&id=YoWO0MJ&id=JboVK8Y&id=9bbRG02&id=QRgjAJR&id=gWaBbkW&id=79q1xY7&id=51o1r9J&id=AMJ7d3k&id=O6ZKK0V&id=RpVzBx1&id=VPdaGdQ&id=lv7Obol&id=GQpBNxw&id=83xb96v&id=BJoM2Qx&id=bQoMKlK&id=8E1QjMW&id=O5OVlK4&id=63Gz0yQ&user_id=aNzoj",

  "/v1/full/playlists?id=K99x4MB&id=Akkz9wv&id=kKdggMN&id=k2J9Na3&id=NVr4gVN&id=1E87V7Q&user_id=aNzoj",

  "/v1/full/tracks/OvyMAV1/reposts?limit=15&offset=0&user_id=aNzoj",
  "/v1/full/tracks/0dgQM/favorites?limit=15&offset=0&user_id=aNzoj",

  "/v1/full/users/PWgX8NR/followers?limit=15&offset=0&user_id=aNzoj",
  "/v1/full/users/PWgX8NR/following?limit=15&offset=0&user_id=aNzoj",
  "/v1/full/users/PWgX8NR/mutuals?limit=5&offset=0&user_id=aNzoj",

  "/v1/full/users/7KVbP/supporting?limit=100&offset=0&user_id=aNzoj",
  "/v1/full/users/7KVbP/supporters?limit=100&offset=0&user_id=aNzoj",

  "/v1/full/playlists/P5abMZp/reposts?limit=15&offset=0&user_id=aNzoj",
  "/v1/full/playlists/P5abMZp/favorites?limit=15&offset=0&user_id=aNzoj",

  "/v1/users?id=0EoAm&user_id=aNzoj",
  "/v1/users?id=0EoAm&id=7eP5n&id=aWvp71&id=Wem1e&user_id=aNzoj",

  "/v1/tracks?id=ZaXp01y&user_id=aNzoj",
  "/v1/tracks?id=ZaXp01y&id=5zdbywJ&id=Pb4MbAX&id=YVog04p&id=7Mv7ZMP&id=g5yzMy2&id=P7dlqNR&id=RR7krAG&id=O7OdxYV&id=72oXyvr&id=jYWyGG0&id=W6KOm8k&id=Xj554dq&id=gbOmq7k&id=G0z4Ap7&id=Yo0XYMm&id=80jlRwP&id=X63z9Wq&id=lZYWRKb&id=JEz6V0o&id=JZq6550&id=b9rbNdM&id=yy71xr7&id=JGNob5Z&id=VpxQ2Aa&id=qGbEZdz&id=B94qO0j&id=oRGgBka&id=jzwlVj0&id=Y5PNmOr&id=5zRm7dx&id=r3KjOg&id=WVP8qav&id=Nvayj1Y&id=V6GZJ3b&id=YoWO0MJ&id=JboVK8Y&id=9bbRG02&id=QRgjAJR&id=gWaBbkW&id=79q1xY7&id=51o1r9J&id=AMJ7d3k&id=O6ZKK0V&id=RpVzBx1&id=VPdaGdQ&id=lv7Obol&id=GQpBNxw&id=83xb96v&id=BJoM2Qx&id=bQoMKlK&id=8E1QjMW&id=O5OVlK4&id=63Gz0yQ&user_id=aNzoj",

  "/v1/playlists?id=K99x4MB&id=Akkz9wv&id=kKdggMN&id=k2J9Na3&id=NVr4gVN&id=1E87V7Q&user_id=aNzoj",

  "/v1/tracks/OvyMAV1/reposts?limit=15&offset=0&user_id=aNzoj",
  "/v1/tracks/0dgQM/favorites?limit=15&offset=0&user_id=aNzoj",

  "/v1/users/PWgX8NR/followers?limit=15&offset=0&user_id=aNzoj",
  "/v1/users/PWgX8NR/following?limit=15&offset=0&user_id=aNzoj",
  "/v1/users/PWgX8NR/mutuals?limit=5&offset=0&user_id=aNzoj",

  "/v1/users/7KVbP/supporting?limit=100&offset=0&user_id=aNzoj",
  "/v1/users/7KVbP/supporters?limit=100&offset=0&user_id=aNzoj",

  "/v1/playlists/P5abMZp/reposts?limit=15&offset=0&user_id=aNzoj",
  "/v1/playlists/P5abMZp/favorites?limit=15&offset=0&user_id=aNzoj",

  "/v1/developer_apps/7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6",
];

import { html } from "https://deno.land/x/html/mod.ts";
import * as jsondiffpatch from "https://esm.sh/jsondiffpatch@0.6.0";
import * as htmlFormatter from "https://esm.sh/jsondiffpatch@0.6.0/formatters/html";

Deno.serve({ port: 62185 }, async (req) => {
  const reqUrl = new URL(req.url);
  const idx = parseInt(reqUrl.searchParams.get("idx") || "0");
  const path = testPaths[idx];
  if (!path || path == "/favicon.ico") return new Response("not found");

  const [r1, r2] = await Promise.all([
    fetchJson("https://discoveryprovider.audius.co" + path),
    fetchJson("http://localhost:1323" + path),
  ]);

  const json1 = r1.data;
  const json2 = r2.data;

  const delta = jsondiffpatch.diff(json1, json2);

  return new Response(
    html`
      <!DOCTYPE html>
      <html lang="en">
        <head>
          <meta charset="UTF-8" />
          <meta
            name="viewport"
            content="width=device-width, initial-scale=1.0"
          />
          <title>APIDIFF</title>
          <link
            rel="stylesheet"
            href="https://esm.sh/jsondiffpatch@0.6.0/lib/formatters/styles/html.css"
            type="text/css"
          />
          <style>
            del {
              background: #ffbbbb;
              text-decoration: line-through;
            }
            ins {
              background: #bbffbb;
            }
          </style>
        </head>
        <body>
          <div style="display: flex; gap: 10px;">
            <select
              onchange="location.href='?idx=' + this.value"
              style="width: 500px"
            >
              ${testPaths.map(
                (path, i) =>
                  html`<option value="${i}" ${i === idx ? "selected" : ""}>
                    ${path}
                  </option>`
              )}
            </select>
            <a href="?idx=${idx - 1}">prev</a>
            <a href="?idx=${idx + 1}">next</a>
          </div>

          <pre>${path}</pre>
          <div>
            <del>${r1.took}ms</del>
            <ins>${r2.took}ms</ins>
          </div>
          <div>${htmlFormatter.format(delta, json1)}</div>
        </body>
      </html>
    `,
    {
      status: 200,
      headers: {
        "content-type": "text/html",
      },
    }
  );
});

async function fetchJson(url: string) {
  const start = performance.now();
  const response = await fetch(url);
  const took = (performance.now() - start).toFixed(0);
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: ${response.statusText}`);
  }
  const resp = await response.json();
  return {
    data: resp.data,
    took,
  };
}
