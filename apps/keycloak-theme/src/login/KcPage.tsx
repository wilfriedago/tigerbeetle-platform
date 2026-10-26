import { lazy, Suspense } from 'react'

import DefaultPage from 'keycloakify/login/DefaultPage'

import { classes } from './classes'
import { useI18n } from './i18n'
import { type KcContext } from './KcContext'
import { ErrorPage, Info, LoginPageExpired, LoginResetPassword, LoginUpdatePassword, LoginVerifyEmail } from './pages/account'
import { Login, LoginPassword, LoginUsername } from './pages/sign-in'
import { LoginConfigTotp, LoginOtp } from './pages/two-factor'
import Template from './template'

const UserProfileFormFields = lazy(async () => import('keycloakify/login/UserProfileFormFields'))

// Keycloak's CSS stays out: every page is drawn by Template with the design tokens.
const doUseDefaultCss = false

/**
 * One page per Keycloak template (pageId). The pages of .design/auth.pen are the platform's own;
 * every other one is Keycloakify's default page inside the platform's Template. Passkey sign-in is
 * one of those, until the design covers it.
 */
export default function KcPage({ kcContext }: { kcContext: KcContext }) {
  const { i18n } = useI18n({ kcContext })
  const common = { doUseDefaultCss, i18n, Template }
  return (
    <Suspense>
      {(() => {
        switch (kcContext.pageId) {
          case 'login.ftl': {
            return kcContext.enableWebAuthnConditionalUI ? null : <Login kcContext={kcContext} {...common} />
          }
          case 'login-username.ftl': {
            return kcContext.enableWebAuthnConditionalUI ? null : <LoginUsername kcContext={kcContext} {...common} />
          }
          case 'login-password.ftl': {
            return kcContext.enableWebAuthnConditionalUI ? null : <LoginPassword kcContext={kcContext} {...common} />
          }
          case 'login-otp.ftl': {
            return <LoginOtp kcContext={kcContext} {...common} />
          }
          case 'login-config-totp.ftl': {
            return <LoginConfigTotp kcContext={kcContext} {...common} />
          }
          case 'login-reset-password.ftl': {
            return <LoginResetPassword kcContext={kcContext} {...common} />
          }
          case 'login-update-password.ftl': {
            return <LoginUpdatePassword kcContext={kcContext} {...common} />
          }
          case 'login-verify-email.ftl': {
            return <LoginVerifyEmail kcContext={kcContext} {...common} />
          }
          case 'info.ftl': {
            return <Info kcContext={kcContext} {...common} />
          }
          case 'error.ftl': {
            return <ErrorPage kcContext={kcContext} {...common} />
          }
          case 'login-page-expired.ftl': {
            return <LoginPageExpired kcContext={kcContext} {...common} />
          }
          default: {
            return null
          }
        }
      })() ?? (
        <DefaultPage
          kcContext={kcContext}
          i18n={i18n}
          classes={classes}
          Template={Template}
          doUseDefaultCss={doUseDefaultCss}
          UserProfileFormFields={UserProfileFormFields}
          doMakeUserConfirmPassword
        />
      )}
    </Suspense>
  )
}
