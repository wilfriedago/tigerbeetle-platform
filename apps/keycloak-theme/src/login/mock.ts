import { createGetKcContextMock } from 'keycloakify/login/KcContext'

import { kcEnvDefaults, themeNames } from '../kc.gen'
import { type KcContextExtension, type KcContextExtensionPerPage } from './KcContext'

// A Keycloak context for `vp dev`: open http://localhost:5174/?page=login-username.ftl to see a
// page without Keycloak (docs.keycloakify.dev, testing your theme).

const kcContextExtension: KcContextExtension = { properties: { ...kcEnvDefaults }, themeName: themeNames[0] ?? 'tbp' }
const kcContextExtensionPerPage: KcContextExtensionPerPage = {}

export const { getKcContextMock } = createGetKcContextMock({
  kcContextExtension,
  kcContextExtensionPerPage,
  overrides: { realm: { displayName: 'TigerBeetle Platform', name: 'tbp' } },
  overridesPerPage: {
    'info.ftl': {
      actionUri: '#accept',
      message: { summary: 'By clicking on the link below, you will become a member of the Acme organization:', type: 'info' }
    },
    'login-password.ftl': { auth: { attemptedUsername: 'ada@acme.example', showUsername: true } },
    'login-verify-email.ftl': { user: { email: 'maria@northwind.example' } }
  }
})
