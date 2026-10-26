import { useState } from 'react'

import { kcSanitize } from 'keycloakify/lib/kcSanitize'
import { type PageProps } from 'keycloakify/login/pages/PageProps'

import { type I18n } from '../i18n'
import { type KcContext } from '../KcContext'
import { Field, inputClass, linkClass, PasswordInput, primaryButton, secondaryButton } from '../ui'

// .design/auth.pen, "Keycloak: forgot password", "set a new password", "verify your email",
// "accept invitation, set password" and "invitation expired". A person the platform invited
// without an account follows one email to verify the address, then sets a password here
// (ADR-0027); a person with an account follows Keycloak's invitation to info.ftl.

type Context<P extends KcContext['pageId']> = Extract<KcContext, { pageId: P }>

export function LoginResetPassword({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-reset-password.ftl'>, I18n>) {
  const { url, auth, messagesPerField } = kcContext
  const error = messagesPerField.existsError('username') ? messagesPerField.getFirstError('username') : undefined
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!error}
      headerNode="Reset your password"
    >
      <p className="-mt-3 text-small text-muted-foreground">
        Enter the email you sign in with. If it has an account, we send a link that works once, for a short time.
      </p>
      <form id="kc-reset-password-form" action={url.loginAction} method="post" className="flex flex-col gap-4">
        <Field id="username" label="Email" error={error}>
          <input
            id="username"
            name="username"
            type="text"
            autoComplete="username"
            className={inputClass}
            defaultValue={auth.attemptedUsername ?? ''}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'username-error' : undefined}
          />
        </Field>
        <button type="submit" className={primaryButton}>
          Send reset link
        </button>
        <a href={url.loginUrl} className={`${linkClass} self-center`}>
          {i18n.msg('backToLogin')}
        </a>
      </form>
    </Template>
  )
}

export function LoginUpdatePassword({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-update-password.ftl'>, I18n>) {
  const { url, messagesPerField, isAppInitiatedAction } = kcContext
  const [sending, setSending] = useState(false)
  const passwordError = messagesPerField.existsError('password') ? messagesPerField.getFirstError('password') : undefined
  const confirmError = messagesPerField.existsError('password-confirm') ? messagesPerField.getFirstError('password-confirm') : undefined
  // Someone signing in for the first time sets a password; anyone else replaces theirs.
  const firstTime = !isAppInitiatedAction && kcContext.auth?.attemptedUsername === undefined
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!passwordError && !confirmError}
      headerNode={firstTime ? 'Set your password' : 'Choose a new password'}
    >
      <p className="-mt-3 text-small text-muted-foreground">
        {firstTime
          ? 'Your account is ready. Choose a password to finish; you sign in with it from now on.'
          : 'Other sessions can be signed out when you save.'}
      </p>
      <form id="kc-passwd-update-form" action={url.loginAction} method="post" className="flex flex-col gap-4" onSubmit={() => setSending(true)}>
        <Field id="password-new" label={i18n.msg('passwordNew')} error={passwordError}>
          <PasswordInput id="password-new" name="password-new" autoComplete="new-password" invalid={Boolean(passwordError)} />
        </Field>
        <Field id="password-confirm" label={i18n.msg('passwordConfirm')} error={confirmError}>
          <PasswordInput id="password-confirm" name="password-confirm" autoComplete="new-password" invalid={Boolean(confirmError)} />
        </Field>
        {firstTime ? null : (
          <label className="flex items-center gap-2 text-small text-foreground">
            <input type="checkbox" id="logout-sessions" name="logout-sessions" value="on" defaultChecked className="accent-primary" />
            {i18n.msg('logoutOtherSessions')}
          </label>
        )}
        <button type="submit" className={primaryButton} disabled={sending}>
          {firstTime ? 'Save and continue' : 'Save and sign in'}
        </button>
        {isAppInitiatedAction ? (
          <button type="submit" name="cancel-aia" value="true" className={secondaryButton}>
            {i18n.msg('doCancel')}
          </button>
        ) : null}
      </form>
    </Template>
  )
}

export function LoginVerifyEmail({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-verify-email.ftl'>, I18n>) {
  const { url, user } = kcContext
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={false}
      headerNode="Check your email"
    >
      <p className="text-small text-muted-foreground">
        We sent a link to <span className="text-foreground">{user?.email}</span>. Open it to verify your address; the roles waiting for you apply once
        it is verified.
      </p>
      <p className="text-small text-muted-foreground">
        Nothing arrived after a few minutes?{' '}
        <a href={url.loginAction} className={linkClass}>
          Send again
        </a>
      </p>
    </Template>
  )
}

export function Info({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'info.ftl'>, I18n>) {
  const { messageHeader, message, requiredActions, skipLink, pageRedirectUri, actionUri, client } = kcContext
  const { advancedMsgStr, msg } = i18n
  // Keycloak's invitation to an organization says "...become a member of the Acme organization";
  // the design titles it "Join Acme".
  const joining = /member of the (?<organization>.+?) organization/i.exec(message.summary)?.groups?.organization
  const invitation = joining !== undefined && actionUri !== undefined
  // The link of an account setup email first lists what is left to do.
  const setup = requiredActions !== undefined && requiredActions.length > 0 && actionUri !== undefined
  const header = messageHeader ? advancedMsgStr(messageHeader) : message.summary
  let html = message.summary.trim()
  if (requiredActions) {
    html += ` <b>${requiredActions.map((a) => advancedMsgStr(`requiredAction.${a}`)).join(', ')}</b>`
  }
  // Keycloak's organization invitation ends its message with the link to accept it.
  const action =
    skipLink || !actionUri
      ? undefined
      : { href: actionUri, label: invitation ? 'Accept invitation' : setup ? 'Continue' : i18n.msgStr('proceedWithAction').replace(/^»\s*/, '') }
  const back = skipLink ? undefined : (pageRedirectUri ?? client.baseUrl)
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={false}
      headerNode={
        invitation ? `Join ${joining}` : setup ? 'Finish setting up your account' : <span dangerouslySetInnerHTML={{ __html: kcSanitize(header) }} />
      }
    >
      {setup ? (
        <div className="flex flex-col gap-2 text-small text-muted-foreground">
          <p>An account was created for you. Two steps are left:</p>
          <ul className="flex list-disc flex-col gap-1 pl-5 text-foreground">
            {requiredActions.map((a) => (
              <li key={a}>{advancedMsgStr(`requiredAction.${a}`)}</li>
            ))}
          </ul>
        </div>
      ) : !invitation && header === message.summary && !requiredActions ? null : (
        <p className="text-small text-muted-foreground" dangerouslySetInnerHTML={{ __html: kcSanitize(html) }} />
      )}
      {action ? (
        <a href={action.href} className={primaryButton}>
          {action.label}
        </a>
      ) : back ? (
        <a href={back} className={primaryButton}>
          {msg('backToApplication')}
        </a>
      ) : null}
      {action ? (
        <p className="text-caption text-muted-foreground">Not expecting this? Ignore the email; nothing is shared until you accept.</p>
      ) : null}
    </Template>
  )
}

/** A page Keycloak shows when a link or a step cannot go on, such as an expired invitation. */
export function ErrorPage({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'error.ftl'>, I18n>) {
  const { message, client, skipLink, url } = kcContext
  const expired = /expired|no longer valid/i.test(message.summary)
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={false}
      headerNode={expired ? 'This link has expired' : i18n.msg('errorTitle')}
    >
      {expired ? (
        <p className="text-small text-muted-foreground">
          Invitations and account links last a few days. Ask the person who invited you to send a new one from the Members page of their organization.
        </p>
      ) : (
        <p className="text-small text-muted-foreground" dangerouslySetInnerHTML={{ __html: kcSanitize(message.summary) }} />
      )}
      {skipLink ? null : (
        <a href={client?.baseUrl ?? url.loginUrl} className={primaryButton}>
          {client?.baseUrl ? i18n.msg('backToApplication') : 'Go to sign in'}
        </a>
      )}
    </Template>
  )
}

export function LoginPageExpired({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-page-expired.ftl'>, I18n>) {
  const { url } = kcContext
  return (
    <Template kcContext={kcContext} i18n={i18n} doUseDefaultCss={doUseDefaultCss} classes={classes ?? {}} headerNode={i18n.msg('pageExpiredTitle')}>
      <p className="text-small text-muted-foreground">The page was open too long. Start again, or carry on where you were.</p>
      <a href={url.loginRestartFlowUrl} className={primaryButton}>
        Start again
      </a>
      <a href={url.loginAction} className={secondaryButton}>
        Carry on
      </a>
    </Template>
  )
}
