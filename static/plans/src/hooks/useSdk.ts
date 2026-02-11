import { sdk } from "@audius/sdk";

const apiKey =
  (import.meta.env.VITE_AUDIUS_API_KEY as string | undefined) ??
  "8acf5eb7436ea403ee536a7334faa5e9ada4b50f";

const instance = sdk({
  appName: "Audius API Plans",
  apiKey,
});

export const useSdk = () => ({ sdk: instance });
