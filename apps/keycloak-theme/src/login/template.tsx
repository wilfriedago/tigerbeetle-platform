import { useEffect } from 'react'

import { kcSanitize } from 'keycloakify/lib/kcSanitize'
import { useInitialize } from 'keycloakify/login/Template.useInitialize'
import { type TemplateProps } from 'keycloakify/login/TemplateProps'

import { type I18n } from './i18n'
import { type KcContext } from './KcContext'
import { Alert, linkClass } from './ui'

// .design/auth.pen, the "Keycloak:" frames: the brand over a centered card on the page background,
// the realm and the language under it. Keycloakify passes every page through this template; the
// props are Keycloakify's own (TemplateProps), so its default pages render inside it too.

export default function Template(props: TemplateProps<KcContext, I18n>) {
  const {
    displayInfo = false,
    displayMessage = true,
    headerNode,
    socialProvidersNode = null,
    infoNode = null,
    documentTitle,
    kcContext,
    i18n,
    doUseDefaultCss,
    children
  } = props
  const { msg, msgStr, currentLanguage, enabledLanguages } = i18n
  const { realm, auth, url, message, isAppInitiatedAction } = kcContext

  useEffect(() => {
    document.title = documentTitle ?? msgStr('loginTitle', realm.displayName || realm.name)
  }, [documentTitle, msgStr, realm.displayName, realm.name])

  const { isReadyToRender } = useInitialize({ doUseDefaultCss, kcContext })
  if (!isReadyToRender) {
    return null
  }

  // After the first step of an identity-first sign-in, the card names the account instead of a
  // title, with a way back to change it.
  const showUsername = auth !== undefined && auth.showUsername && !auth.showResetCredentials

  return (
    <div className="flex min-h-svh flex-col items-center gap-7 bg-background px-4 pt-[min(120px,12vh)] pb-10">
      <Brand />
      <main className="flex w-full max-w-[400px] flex-col gap-5 rounded-lg border border-border bg-surface p-7 shadow-sm">
        {showUsername ? (
          <div className="flex flex-col gap-2">
            <h1 className="text-section-title text-foreground">{headerNode}</h1>
            <div className="flex items-center justify-between gap-3 rounded-md border border-border-subtle bg-surface-sunken px-3 py-2">
              <span className="truncate text-small text-foreground">{auth.attemptedUsername}</span>
              <a href={url.loginRestartFlowUrl} className={linkClass} aria-label={msgStr('restartLoginTooltip')}>
                Change
              </a>
            </div>
          </div>
        ) : (
          <h1 className="text-section-title text-foreground">{headerNode}</h1>
        )}
        {/* App-initiated actions do not repeat the warning that asked for them. */}
        {displayMessage && message !== undefined && (message.type !== 'warning' || !isAppInitiatedAction) ? (
          <Alert type={message.type} html={kcSanitize(message.summary)} />
        ) : null}
        {children}
        {auth?.showTryAnotherWayLink ? (
          <form id="kc-select-try-another-way-form" action={url.loginAction} method="post">
            <input type="hidden" name="tryAnotherWay" value="on" />
            <button type="submit" className={linkClass}>
              {msg('doTryAnotherWay')}
            </button>
          </form>
        ) : null}
        {socialProvidersNode}
        {displayInfo ? <div className="border-t border-border-subtle pt-4 text-small text-muted-foreground">{infoNode}</div> : null}
      </main>
      <footer className="flex items-center gap-2 text-caption text-subtle-foreground">
        <span>
          Keycloak realm <span className="font-mono">{realm.name}</span>
        </span>
        <span aria-hidden="true">·</span>
        {enabledLanguages.length > 1 ? (
          <select
            aria-label={msgStr('languages')}
            value={currentLanguage.languageTag}
            onChange={(event) => {
              const next = enabledLanguages.find((l) => l.languageTag === event.target.value)
              if (next) {
                globalThis.location.assign(next.href)
              }
            }}
            className="cursor-pointer rounded-sm bg-transparent text-caption text-subtle-foreground outline-none hover:text-foreground focus-visible:ring-3 focus-visible:ring-focus-ring"
          >
            {enabledLanguages.map(({ languageTag, label }) => (
              <option key={languageTag} value={languageTag}>
                {label}
              </option>
            ))}
          </select>
        ) : (
          <span>{languageName(currentLanguage.languageTag, currentLanguage.label)}</span>
        )}
      </footer>
    </div>
  )
}

/** The platform's mark and name, as in the console's sign-in screens. */
function Brand() {
  return (
    <div className="flex items-center gap-2.5">
      <span
        aria-hidden="true"
        className="flex size-7 items-center justify-center rounded-md bg-primary font-mono text-small font-medium text-primary-foreground"
      >
        tb
      </span>
      <span className="font-mono text-body font-medium text-foreground">tigerbeetle-platform</span>
    </div>
  )
}

/** "English" for en: Keycloak labels the language with its tag when the realm has one language. */
function languageName(tag: string, label: string): string {
  if (label !== tag) {
    return label
  }
  try {
    return new Intl.DisplayNames([tag], { type: 'language' }).of(tag) ?? label
  } catch {
    return label
  }
}
