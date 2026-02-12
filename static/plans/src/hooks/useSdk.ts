import { sdk } from "@audius/sdk";

const apiKey =
  (import.meta.env.VITE_AUDIUS_API_KEY as string | undefined) ??
  "2cc593fc814461263d282a84286fd4f72c79562e";

const instance = sdk({
  appName: "Audius API Plans",
  apiKey,
});

export const useSdk = () => ({ sdk: instance });
