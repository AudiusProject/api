// npx deno run -A apidiff.ts

import { assertEquals } from "jsr:@std/assert";

async function fetchJson(url: string): Promise<any> {
  const response = await fetch(url);
  if (!response.ok) {
    throw new Error(`Failed to fetch ${url}: ${response.statusText}`);
  }
  const resp = await response.json();
  if (Array.isArray(resp.data)) {
    const keyedData = resp.data.reduce(
      (acc: Record<string, any>, item: any) => {
        const id = item.id || item.receiver?.id;
        if (!id) {
          console.log("unable to key", item);
          throw new Error(`unable to key item`);
        }
        acc[id] = item;
        return acc;
      },
      {}
    );
    return keyedData;
  }
  return resp.data;
}

async function compareApis(path: string): Promise<void> {
  const [json1, json2] = await Promise.all([
    fetchJson("https://discoveryprovider.audius.co/" + path),
    fetchJson("http://localhost:1323" + path),
  ]);

  try {
    assertEquals(json1, json2);
  } catch (e) {
    console.log(e);
  }
}

const testPaths = [
  // "/v1/full/users?id=0EoAm&user_id=aNzoj",
  // "/v1/full/users?id=0EoAm&id=7eP5n&id=aWvp71&id=Wem1e&user_id=aNzoj",

  // "/v1/full/tracks?id=ZaXp01y&user_id=aNzoj",
  // "/v1/full/tracks?id=ZaXp01y&id=5zdbywJ&id=Pb4MbAX&id=YVog04p&id=7Mv7ZMP&id=g5yzMy2&id=P7dlqNR&id=RR7krAG&id=O7OdxYV&id=72oXyvr&id=jYWyGG0&id=W6KOm8k&id=Xj554dq&id=gbOmq7k&id=G0z4Ap7&id=Yo0XYMm&id=80jlRwP&id=X63z9Wq&id=lZYWRKb&id=JEz6V0o&id=JZq6550&id=b9rbNdM&id=yy71xr7&id=JGNob5Z&id=VpxQ2Aa&id=qGbEZdz&id=B94qO0j&id=oRGgBka&id=jzwlVj0&id=Y5PNmOr&id=5zRm7dx&id=r3KjOg&id=WVP8qav&id=Nvayj1Y&id=V6GZJ3b&id=YoWO0MJ&id=JboVK8Y&id=9bbRG02&id=QRgjAJR&id=gWaBbkW&id=79q1xY7&id=51o1r9J&id=AMJ7d3k&id=O6ZKK0V&id=RpVzBx1&id=VPdaGdQ&id=lv7Obol&id=GQpBNxw&id=83xb96v&id=BJoM2Qx&id=bQoMKlK&id=8E1QjMW&id=O5OVlK4&id=63Gz0yQ&user_id=aNzoj",

  // "/v1/full/playlists?id=K99x4MB&id=Akkz9wv&id=kKdggMN&id=k2J9Na3&id=NVr4gVN&id=1E87V7Q&user_id=aNzoj",

  // "/v1/full/tracks/OvyMAV1/reposts?limit=15&offset=0&user_id=aNzoj",
  // "/v1/full/tracks/0dgQM/favorites?limit=15&offset=0&user_id=aNzoj",

  // "/v1/full/users/PWgX8NR/followers?limit=15&offset=0&user_id=aNzoj",
  // "/v1/full/users/PWgX8NR/following?limit=15&offset=0&user_id=aNzoj",
  // "/v1/full/users/PWgX8NR/mutuals?limit=5&offset=0&user_id=aNzoj",

  // "/v1/full/users/7KVbP/supporting?limit=100&offset=0&user_id=aNzoj",
  // "/v1/full/users/7KVbP/supporters?limit=100&offset=0&user_id=aNzoj",

  "/v1/full/playlists/P5abMZp/reposts?limit=15&offset=0&user_id=aNzoj",
  "/v1/full/playlists/P5abMZp/favorites?limit=15&offset=0&user_id=aNzoj",

  // "/v1/users?id=0EoAm&user_id=aNzoj",
  // "/v1/users?id=0EoAm&id=7eP5n&id=aWvp71&id=Wem1e&user_id=aNzoj",

  // "/v1/tracks?id=ZaXp01y&user_id=aNzoj",
  // "/v1/tracks?id=ZaXp01y&id=5zdbywJ&id=Pb4MbAX&id=YVog04p&id=7Mv7ZMP&id=g5yzMy2&id=P7dlqNR&id=RR7krAG&id=O7OdxYV&id=72oXyvr&id=jYWyGG0&id=W6KOm8k&id=Xj554dq&id=gbOmq7k&id=G0z4Ap7&id=Yo0XYMm&id=80jlRwP&id=X63z9Wq&id=lZYWRKb&id=JEz6V0o&id=JZq6550&id=b9rbNdM&id=yy71xr7&id=JGNob5Z&id=VpxQ2Aa&id=qGbEZdz&id=B94qO0j&id=oRGgBka&id=jzwlVj0&id=Y5PNmOr&id=5zRm7dx&id=r3KjOg&id=WVP8qav&id=Nvayj1Y&id=V6GZJ3b&id=YoWO0MJ&id=JboVK8Y&id=9bbRG02&id=QRgjAJR&id=gWaBbkW&id=79q1xY7&id=51o1r9J&id=AMJ7d3k&id=O6ZKK0V&id=RpVzBx1&id=VPdaGdQ&id=lv7Obol&id=GQpBNxw&id=83xb96v&id=BJoM2Qx&id=bQoMKlK&id=8E1QjMW&id=O5OVlK4&id=63Gz0yQ&user_id=aNzoj",

  // "/v1/playlists?id=K99x4MB&id=Akkz9wv&id=kKdggMN&id=k2J9Na3&id=NVr4gVN&id=1E87V7Q&user_id=aNzoj",

  // "/v1/tracks/OvyMAV1/reposts?limit=15&offset=0&user_id=aNzoj",
  // "/v1/tracks/0dgQM/favorites?limit=15&offset=0&user_id=aNzoj",

  // "/v1/users/PWgX8NR/followers?limit=15&offset=0&user_id=aNzoj",
  // "/v1/users/PWgX8NR/following?limit=15&offset=0&user_id=aNzoj",
  // "/v1/users/PWgX8NR/mutuals?limit=5&offset=0&user_id=aNzoj",

  // "/v1/users/7KVbP/supporting?limit=100&offset=0&user_id=aNzoj",
  // "/v1/users/7KVbP/supporters?limit=100&offset=0&user_id=aNzoj",

  // "/v1/playlists/P5abMZp/reposts?limit=15&offset=0&user_id=aNzoj",
  // "/v1/playlists/P5abMZp/favorites?limit=15&offset=0&user_id=aNzoj",

  // "/v1/developer_apps/7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6"
];

for (const path of testPaths) {
  await compareApis(path);
}
