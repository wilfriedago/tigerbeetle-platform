import { useState } from 'react'

import { type PageProps } from 'keycloakify/login/pages/PageProps'

import { type I18n } from '../i18n'
import { type KcContext } from '../KcContext'
import { Field, inputClass, primaryButton, secondaryButton } from '../ui'

// .design/auth.pen, "Keycloak: second factor" and "Keycloak: set up two-factor".

type Context<P extends KcContext['pageId']> = Extract<KcContext, { pageId: P }>

export function LoginOtp({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-otp.ftl'>, I18n>) {
  const { url, otpLogin, messagesPerField } = kcContext
  const [sending, setSending] = useState(false)
  const error = messagesPerField.existsError('totp') ? messagesPerField.getFirstError('totp') : undefined
  const credentials = otpLogin.userOtpCredentials
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!error}
      displayInfo
      infoNode={<p>Lost your device? Ask an owner of your organization to have a new second factor set up.</p>}
      headerNode="Two-factor code"
    >
      <p className="-mt-3 text-small text-muted-foreground">Enter the code from your authenticator app.</p>
      <form id="kc-otp-login-form" action={url.loginAction} method="post" className="flex flex-col gap-4" onSubmit={() => setSending(true)}>
        {credentials.length > 1 ? (
          <fieldset className="flex flex-col gap-1.5">
            <legend className="mb-1.5 text-small font-medium text-foreground">Device</legend>
            {credentials.map((c) => (
              <label key={c.id} className="flex items-center gap-2 text-body text-foreground">
                <input
                  type="radio"
                  name="selectedCredentialId"
                  value={c.id}
                  defaultChecked={c.id === otpLogin.selectedCredentialId}
                  className="accent-primary"
                />
                {c.userLabel}
              </label>
            ))}
          </fieldset>
        ) : null}
        <Field id="otp" label="Code" error={error}>
          <input
            id="otp"
            name="otp"
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            className={`${inputClass} font-mono tracking-[0.4em]`}
            aria-invalid={error ? true : undefined}
            aria-describedby={error ? 'otp-error' : undefined}
          />
        </Field>
        <button id="kc-login" name="login" type="submit" className={primaryButton} disabled={sending}>
          Verify
        </button>
      </form>
    </Template>
  )
}

export function LoginConfigTotp({ kcContext, i18n, doUseDefaultCss, Template, classes }: PageProps<Context<'login-config-totp.ftl'>, I18n>) {
  const { url, isAppInitiatedAction, totp, mode, messagesPerField } = kcContext
  const totpError = messagesPerField.existsError('totp') ? messagesPerField.getFirstError('totp') : undefined
  const labelError = messagesPerField.existsError('userLabel') ? messagesPerField.getFirstError('userLabel') : undefined
  const manual = mode === 'manual'
  return (
    <Template
      kcContext={kcContext}
      i18n={i18n}
      doUseDefaultCss={doUseDefaultCss}
      classes={classes ?? {}}
      displayMessage={!totpError && !labelError}
      headerNode="Set up two-factor authentication"
    >
      <p className="-mt-3 text-small text-muted-foreground">Protects your account if your password leaks. It takes a minute.</p>
      <ol className="flex flex-col gap-3 text-small text-foreground">
        <li className="flex gap-2.5">
          <Step n={1} />
          <span>Install an authenticator app such as {totp.supportedApplications.map((app) => i18n.advancedMsgStr(app)).join(' or ')}.</span>
        </li>
        <li className="flex gap-2.5">
          <Step n={2} />
          <span className="flex flex-col gap-2">
            {manual ? (
              <>
                <span>Enter this key in the app.</span>
                <code className="rounded-md border border-border-subtle bg-surface-sunken px-2.5 py-1.5 font-mono text-small break-all">
                  {totp.totpSecretEncoded}
                </code>
                <a href={totp.qrUrl} className="text-primary-text hover:underline">
                  Scan a code instead
                </a>
              </>
            ) : (
              <>
                <span>Scan the code.</span>
                <img
                  src={`data:image/png;base64, ${totp.totpSecretQrCode}`}
                  alt="QR code of your two-factor key"
                  className="size-40 rounded-md border border-border bg-white p-1.5"
                />
                <a href={totp.manualUrl} className="text-primary-text hover:underline">
                  Cannot scan? Enter the key instead
                </a>
              </>
            )}
          </span>
        </li>
        <li className="flex gap-2.5">
          <Step n={3} />
          <span>Type the code the app shows.</span>
        </li>
      </ol>
      <form id="kc-totp-settings-form" action={url.loginAction} method="post" className="flex flex-col gap-4">
        <Field id="totp" label="Code from the app" error={totpError}>
          <input
            id="totp"
            name="totp"
            type="text"
            inputMode="numeric"
            autoComplete="one-time-code"
            className={`${inputClass} font-mono tracking-[0.4em]`}
            aria-invalid={totpError ? true : undefined}
          />
        </Field>
        <input type="hidden" id="totpSecret" name="totpSecret" value={totp.totpSecret} />
        {mode ? <input type="hidden" id="mode" value={mode} /> : null}
        <Field id="userLabel" label="Device name" error={labelError} hint="Helps you recognise it later in your Keycloak account.">
          <input id="userLabel" name="userLabel" type="text" autoComplete="off" className={inputClass} aria-invalid={labelError ? true : undefined} />
        </Field>
        <label className="flex items-center gap-2 text-small text-foreground">
          <input type="checkbox" id="logout-sessions" name="logout-sessions" value="on" defaultChecked className="accent-primary" />
          Sign out other devices
        </label>
        <button type="submit" id="saveTOTPBtn" className={primaryButton}>
          Turn on two-factor
        </button>
        {isAppInitiatedAction ? (
          <button type="submit" id="cancelTOTPBtn" name="cancel-aia" value="true" className={secondaryButton}>
            {i18n.msg('doCancel')}
          </button>
        ) : null}
      </form>
    </Template>
  )
}

function Step({ n }: { n: number }) {
  return (
    <span
      aria-hidden="true"
      className="flex size-5 shrink-0 items-center justify-center rounded-full bg-primary-subtle font-mono text-caption text-primary-text"
    >
      {n}
    </span>
  )
}
