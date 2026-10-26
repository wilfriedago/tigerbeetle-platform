import { useState } from 'react'

import { type PageProps } from 'keycloakify/login/pages/PageProps'

import { type I18n } from '../i18n'
import { type KcContext } from '../KcContext'
import { cn, Field, inputClass, linkClass, PasswordInput, primaryButton, secondaryButton } from '../ui'

// .design/auth.pen, "Keycloak: sign in, email first" and "Keycloak: sign in, password". Once an
// organization exists, Keycloak asks for the email first (identity-first), so the platform's realm
// shows login-username.ftl then login-password.ftl; login.ftl, both on one page, is for a realm
// without organizations.

type Context<P extends KcContext['pageId']> = Extract<KcContext, { pageId: P }>

function SocialProviders({ kcContext }: { kcContext: Context<'login.ftl' | 'login-username.ftl'> }) {
  const providers = kcContext.social?.providers ?? []
  if (!kcContext.realm.password || providers.length === 0) {
    return null
  }
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-3 text-caption text-subtle-foreground">
        <span className="h-px flex-1 bg-border-subtle" />
        or
        <span className="h-px flex-1 bg-border-subtle" />
      </div>
      {providers.map((p) => (
        <a key={p.alias} id={`social-${p.alias}`} href={p.loginUrl} className={secondaryButton}>
          {p.displayName}
        </a>
      ))}
    </div>
  )
}

function NoPublicSignUp({ i18n }: { i18n: I18n }) {
  return <p>{i18n.msg('noPublicSignUp')}</p>
}

/** The label of the username field, as the realm allows it. */
function usernameLabel(kcContext: Context<'login.ftl' | 'login-username.ftl'>, i18n: I18n): string {
  const { realm } = kcContext
  if (!realm.loginWithEmailAllowed) {
    return i18n.msgStr('username')
  }
  return realm.registrationEmailAsUsername ? i18n.msgStr('email') : 'Email'
}

export function LoginUsername({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-username.ftl'>, I18n>) {
  const { url, login, usernameHidden, messagesPerField } = kcContext
  const [sending, setSending] = useState(false)
  const error = messagesPerField.existsError('username') ? messagesPerField.getFirstError('username') : undefined
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!error}
      displayInfo
      infoNode={<NoPublicSignUp i18n={i18n} />}
      headerNode={i18n.msg('doLogIn')}
      socialProvidersNode={<SocialProviders kcContext={kcContext} />}
    >
      <p className="-mt-3 text-small text-muted-foreground">{i18n.msg('signInHint')}</p>
      <form id="kc-form-login" action={url.loginAction} method="post" className="flex flex-col gap-4" onSubmit={() => setSending(true)}>
        {usernameHidden ? null : (
          <Field id="username" label={usernameLabel(kcContext, i18n)} error={error}>
            <input
              id="username"
              name="username"
              type="text"
              className={inputClass}
              defaultValue={login.username ?? ''}
              autoComplete="username"
              aria-invalid={error ? true : undefined}
              aria-describedby={error ? 'username-error' : undefined}
            />
          </Field>
        )}
        <button id="kc-login" name="login" type="submit" className={primaryButton} disabled={sending}>
          {i18n.msg('doContinue')}
        </button>
      </form>
    </Template>
  )
}

export function LoginPassword({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-password.ftl'>, I18n>) {
  const { url, realm, messagesPerField } = kcContext
  const [sending, setSending] = useState(false)
  const error = messagesPerField.existsError('password') ? messagesPerField.getFirstError('password') : undefined
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!error}
      headerNode="Enter your password"
    >
      <form id="kc-form-login" action={url.loginAction} method="post" className="flex flex-col gap-4" onSubmit={() => setSending(true)}>
        <Field
          id="password"
          label={i18n.msg('password')}
          error={error}
          aside={
            realm.resetPasswordAllowed ? (
              <a href={url.loginResetCredentialsUrl} className={linkClass}>
                {i18n.msg('doForgotPassword')}
              </a>
            ) : null
          }
        >
          <PasswordInput id="password" name="password" autoComplete="current-password" invalid={Boolean(error)} />
        </Field>
        <button id="kc-login" name="login" type="submit" className={primaryButton} disabled={sending}>
          {i18n.msg('doLogIn')}
        </button>
      </form>
    </Template>
  )
}

export function Login({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login.ftl'>, I18n>) {
  const { url, realm, login, usernameHidden, messagesPerField, auth } = kcContext
  const [sending, setSending] = useState(false)
  const error = messagesPerField.existsError('username', 'password') ? messagesPerField.getFirstError('username', 'password') : undefined
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!error}
      displayInfo
      infoNode={<NoPublicSignUp i18n={i18n} />}
      headerNode={i18n.msg('doLogIn')}
      socialProvidersNode={<SocialProviders kcContext={kcContext} />}
    >
      {realm.password ? (
        <form id="kc-form-login" action={url.loginAction} method="post" className="flex flex-col gap-4" onSubmit={() => setSending(true)}>
          {usernameHidden ? null : (
            <Field id="username" label={usernameLabel(kcContext, i18n)}>
              <input
                id="username"
                name="username"
                type="text"
                className={inputClass}
                defaultValue={login.username ?? ''}
                autoComplete="username"
                aria-invalid={error ? true : undefined}
              />
            </Field>
          )}
          <Field
            id="password"
            label={i18n.msg('password')}
            error={error}
            aside={
              realm.resetPasswordAllowed ? (
                <a href={url.loginResetCredentialsUrl} className={linkClass}>
                  {i18n.msg('doForgotPassword')}
                </a>
              ) : null
            }
          >
            <PasswordInput id="password" name="password" autoComplete="current-password" invalid={Boolean(error)} />
          </Field>
          <input type="hidden" id="id-hidden-input" name="credentialId" value={auth.selectedCredential ?? ''} />
          <button id="kc-login" name="login" type="submit" className={cn(primaryButton)} disabled={sending}>
            {i18n.msg('doLogIn')}
          </button>
        </form>
      ) : null}
    </Template>
  )
}
