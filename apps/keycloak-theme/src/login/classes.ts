import { type ClassKey } from 'keycloakify/login'

import { inputClass, primaryButton, secondaryButton } from './ui'

// Keycloakify's default pages (those .design/auth.pen does not draw, such as the profile a new
// account fills in) name their parts with Keycloak's class keys; this maps the common ones to the
// design's classes, so they look like the platform's own pages.
export const classes = {
  kcButtonClass: 'inline-flex',
  kcButtonDefaultClass: secondaryButton,
  kcButtonPrimaryClass: primaryButton,
  kcFormButtonsClass: 'flex flex-col gap-2 pt-2',
  kcFormClass: 'flex flex-col gap-4',
  kcFormGroupClass: 'flex flex-col gap-1.5',
  kcFormOptionsClass: 'text-small',
  kcFormPasswordVisibilityButtonClass: 'px-2 text-muted-foreground hover:text-foreground',
  kcInputClass: inputClass,
  kcInputErrorMessageClass: 'text-caption text-danger-fg',
  kcInputGroup: 'flex items-center gap-1',
  kcInputHelperTextAfterClass: 'text-caption text-muted-foreground',
  kcInputHelperTextBeforeClass: 'text-caption text-muted-foreground',
  kcInputWrapperClass: 'flex flex-col gap-1',
  kcLabelClass: 'text-small font-medium text-foreground',
  kcLabelWrapperClass: 'flex'
} satisfies Partial<Record<ClassKey, string>>
