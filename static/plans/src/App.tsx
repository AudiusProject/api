import { useEffect, useRef, useState } from 'react'
import {
  ThemeProvider as HarmonyThemeProvider,
  Text,
  Paper,
  Flex
} from '@audius/harmony'
import { css } from '@emotion/react'
import { useSdk } from './hooks/useSdk'

type User = { userId: string; handle: string }

const messages = {
  title: 'Audius API Plans',
  apiKeysSection: 'Your API Keys',
  loginWithAudius: 'Login with Audius',
  loggedInAs: 'Logged in as:',
  gettingStarted: 'Getting Started',
  apiDescription:
    'The Audius API offers performant access to the Open Audio Protocol (openaudio.org).',
  rest: 'REST:',
  restEndpoint: 'api.audius.co',
  grpc: 'GRPC:',
  grpcEndpoint: 'grpc.audius.co',
  apiDocs: 'API Docs:',
  apiDocsUrl: 'docs.audius.co/api',
  swaggerDefinition: 'Swagger Definition:',
  swaggerUrl: 'api.audius.co/v1',
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
    if (!sdk.oauth) return

    let isMounted = true
    let buttonRendered = false

    sdk.oauth.init({
      successCallback: (user: User) => {
        if (isMounted) {
          setUser(user)
        }
      },
      errorCallback: (error: string) => {
        // Only log non-signing errors to avoid noise
        // The personal_sign errors are expected when no Web3 provider is available
        // The SDK handles these gracefully and continues without signing
        if (
          !error.includes('personal_sign') &&
          !error.includes('RPC') &&
          !error.includes('UnknownRpcError')
        ) {
          console.error('OAuth error:', error)
        }
      }
    })

    // Render button after a short delay to ensure DOM is ready
    const timer = setTimeout(() => {
      if (
        isMounted &&
        loginWithAudiusButtonRef.current &&
        sdk.oauth &&
        !buttonRendered
      ) {
        try {
          sdk.oauth.renderButton({
            element: loginWithAudiusButtonRef.current,
            scope: 'read' // Use 'read' scope to avoid Web3 signing requirements
          })
          buttonRendered = true
        } catch (error) {
          console.error('Error rendering OAuth button:', error)
        }
      }
    }, 100)

    return () => {
      isMounted = false
      clearTimeout(timer)
      // Don't manually clean up the button - let React handle it
      // The SDK manages its own DOM elements
    }
  }, [sdk.oauth])

  // Set page background to Harmony background color
  useEffect(() => {
    document.body.style.backgroundColor = 'var(--harmony-background)'
    document.body.style.minHeight = '100vh'
  }, [])

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
                <div
                  ref={loginWithAudiusButtonRef}
                  css={css`
                    min-height: 40px;
                    display: flex;
                    align-items: center;
                  `}
                />
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

        {/* Getting Started Section */}
        <Paper p='l'>
          <Flex direction='column' gap='m'>
            <Text color='heading' strength='strong' variant='heading'>
              {messages.gettingStarted}
            </Text>
            <Text>
              The Audius API gives performant access to the Open Audio Protocol (
              <Text
                tag='a'
                href='https://openaudio.org'
                target='_blank'
                rel='noopener noreferrer'
                css={css`
                  color: inherit;
                  text-decoration: underline;
                  &:hover {
                    text-decoration: none;
                  }
                `}
              >
                openaudio.org
              </Text>
              ).
            </Text>

              {/* Links Section */}
              <Flex direction='column' gap='xs'>
                <Flex gap='s' alignItems='center' wrap='wrap'>
                  <Text strength='strong'>{messages.apiDocs}</Text>
                  <Text
                    tag='a'
                    href={`https://${messages.apiDocsUrl}`}
                    target='_blank'
                    rel='noopener noreferrer'
                    css={css`
                      color: inherit;
                      text-decoration: underline;
                      &:hover {
                        text-decoration: none;
                      }
                    `}
                  >
                    {messages.apiDocsUrl}
                  </Text>
                </Flex>
                <Flex gap='s' alignItems='center' wrap='wrap'>
                  <Text strength='strong'>{messages.swaggerDefinition}</Text>
                  <Text
                    tag='a'
                    href={`https://${messages.swaggerUrl}`}
                    target='_blank'
                    rel='noopener noreferrer'
                    css={css`
                      color: inherit;
                      text-decoration: underline;
                      &:hover {
                        text-decoration: none;
                      }
                    `}
                  >
                    {messages.swaggerUrl}
                  </Text>
                </Flex>
              </Flex>
            <Flex direction='column' gap='l'>
              {/* REST Section */}
              <Flex direction='column' gap='s'>
                <Flex gap='s' alignItems='center' wrap='wrap'>
                  <Text strength='strong'>{messages.rest}</Text>
                  <Text
                    tag='a'
                    href={`https://${messages.restEndpoint}`}
                    target='_blank'
                    rel='noopener noreferrer'
                    css={css`
                      color: inherit;
                      text-decoration: underline;
                      &:hover {
                        text-decoration: none;
                      }
                    `}
                  >
                    {messages.restEndpoint}
                  </Text>
                </Flex>
                <pre
                  css={css`
                    background: #f5f5f5;
                    padding: 1rem;
                    border-radius: 0.5rem;
                    overflow-x: auto;
                    margin: 0;
                    font-size: 0.875rem;
                    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
                    border: 1px solid #e0e0e0;
                  `}
                >
                  <code>curl https://api.audius.co/v1/tracks/trending</code>
                </pre>
              </Flex>

              {/* GRPC Section */}
              <Flex direction='column' gap='s'>
                <Flex gap='s' alignItems='center' wrap='wrap'>
                  <Text strength='strong'>{messages.grpc}</Text>
                  <Text>{messages.grpcEndpoint}</Text>
                </Flex>
                <pre
                  css={css`
                    background: #f5f5f5;
                    padding: 1rem;
                    border-radius: 0.5rem;
                    overflow-x: auto;
                    margin: 0;
                    font-size: 0.875rem;
                    font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
                    border: 1px solid #e0e0e0;
                  `}
                >
                  <code>grpcurl grpc.audius.co:443 list</code>
                </pre>
              </Flex>
            </Flex>
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
