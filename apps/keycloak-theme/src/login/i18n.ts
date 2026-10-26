import { i18nBuilder } from 'keycloakify/login'

import { type ThemeName } from '../kc.gen'

// Keycloak's own messages, in every language the realm enables, with the platform's wording for
// the few the design changes (docs.keycloakify.dev, i18n).
const { useI18n, ofTypeI18n } = i18nBuilder
  .withThemeName<ThemeName>()
  .withCustomTranslations({
    en: {
      // Sentence case, as everywhere in the console.
      doForgotPassword: 'Forgot password?',
      doLogIn: 'Sign in',
      passwordNew: 'New password',
      doContinue: 'Continue',
      noPublicSignUp: 'No account? Accounts are created when an organization invites you. There is no public sign-up.',
      signInHint: 'Use your work email. If your organization uses single sign-on, you continue there.'
    }
  })
  .build()

type I18n = typeof ofTypeI18n

export { type I18n, useI18n }
