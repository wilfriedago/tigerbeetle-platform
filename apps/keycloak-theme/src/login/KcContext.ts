import { type ExtendKcContext } from 'keycloakify/login'

import { type KcEnvName, type ThemeName } from '../kc.gen'

// Keycloakify's extension points (docs.keycloakify.dev, KcContext): the theme declares no
// environment variable and no page needs more than Keycloak sends, so both resolve to empty
// objects on purpose.
export interface KcContextExtension {
  themeName: ThemeName
  // eslint-disable-next-line typescript/no-generated-empty-object-type -- no environment variable yet
  properties: Record<KcEnvName, string>
}

// eslint-disable-next-line typescript/no-generated-empty-object-type -- no page extension yet
export type KcContextExtensionPerPage = Record<never, never>

export type KcContext = ExtendKcContext<KcContextExtension, KcContextExtensionPerPage>
