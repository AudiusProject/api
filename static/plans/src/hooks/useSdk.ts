import { sdk } from '@audius/sdk'

const instance = sdk({
  appName: 'Audius API Plans',
  apiKey: '8acf5eb7436ea403ee536a7334faa5e9ada4b50f'
})

export const useSdk = () => ({ sdk: instance })
