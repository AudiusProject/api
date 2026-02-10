import { useEffect, useRef, useState } from 'react'
import {
  ThemeProvider as HarmonyThemeProvider,
  Text,
  Paper,
  Button,
  Flex
} from '@audius/harmony'
import { css } from '@emotion/react'
import { useSdk } from './hooks/useSdk'

type User = { userId: string; handle: string }

const messages = {
  title: 'Audius API Plans',
  apiKeysSection: 'Creating & Managing API keys',
  loginWithAudius: 'Login with Audius',
  loggedInAs: 'Logged in as:',
  freePlan: 'Free',
  unlimitedPlan: 'Unlimited',
  rateLimits: 'Rate Limits:',
  requestsPerSecond: 'Requests/Second',
  requestsPerMonth: 'Requests/Month',
  freeRateLimit: '10',
  freeMonthlyLimit: '200,000',
  unlimitedRateLimit: 'Unlimited',
  unlimitedMonthlyLimit: 'Unlimited'
}

export default function App() {
  const { sdk } = useSdk()

  const [user, setUser] = useState<User | null>(null)
  const loginWithAudiusButtonRef = useRef<HTMLDivElement>(null)

  /**
   * Init @audius/sdk oauth
   */
  useEffect(() => {
    sdk.oauth?.init({
      successCallback: (user: User) => setUser(user),
      errorCallback: (error: string) => console.log('Got error', error)
    })

    if (loginWithAudiusButtonRef.current) {
      sdk.oauth?.renderButton({
        element: loginWithAudiusButtonRef.current,
        scope: 'write'
      })
    }
  }, [sdk.oauth])

  return (
    <HarmonyThemeProvider theme='day'>
      <Flex
        direction='column'
        gap='xl'
        m='2xl'
        css={css`
          max-width: 1200px;
          margin: 0 auto;
          padding: 2rem;
        `}
      >
        {/* Header */}
        <Flex direction='column' gap='s'>
          <Text color='heading' strength='strong' variant='display'>
            {messages.title}
          </Text>
        </Flex>

        {/* Login Section */}
        <Paper p='l'>
          <Flex direction='column' gap='m'>
            <Text color='heading' strength='strong' variant='heading'>
              {messages.apiKeysSection}
            </Text>
            {!user ? (
              <Flex gap='m' alignItems='center'>
                <Text>{messages.loginWithAudius}</Text>
                <div ref={loginWithAudiusButtonRef} />
              </Flex>
            ) : (
              <Flex direction='column' gap='s'>
                <Text>
                  {messages.loggedInAs} {user.handle}
                </Text>
                {/* Placeholder for future API key management */}
              </Flex>
            )}
          </Flex>
        </Paper>

        {/* Plans Section */}
        <Flex
          gap='l'
          css={css`
            @media (max-width: 768px) {
              flex-direction: column;
            }
          `}
        >
          {/* Free Plan */}
          <Paper p='l' css={{ flex: 1 }}>
            <Flex direction='column' gap='m'>
              <Text color='heading' strength='strong' variant='heading'>
                {messages.freePlan}
              </Text>
              <Flex direction='column' gap='s'>
                <Text>
                  <strong>{messages.rateLimits}</strong> {messages.freeRateLimit}{' '}
                  {messages.requestsPerSecond}
                </Text>
                <Text>
                  <strong>
                    {messages.freeMonthlyLimit} {messages.requestsPerMonth}
                  </strong>
                </Text>
              </Flex>
            </Flex>
          </Paper>

          {/* Unlimited Plan */}
          <Paper p='l' css={{ flex: 1 }}>
            <Flex direction='column' gap='m'>
              <Text color='heading' strength='strong' variant='heading'>
                {messages.unlimitedPlan}
              </Text>
              <Flex direction='column' gap='s'>
                <Text>
                  <strong>{messages.rateLimits}</strong> {messages.unlimitedRateLimit}{' '}
                  {messages.requestsPerSecond}
                </Text>
                <Text>
                  <strong>
                    {messages.unlimitedMonthlyLimit} {messages.requestsPerMonth}
                  </strong>
                </Text>
              </Flex>
            </Flex>
          </Paper>
        </Flex>
      </Flex>
    </HarmonyThemeProvider>
  )
}
