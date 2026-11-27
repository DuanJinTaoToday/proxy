export const supportFixtureMode = process.env.PROXY_VERIFY_SUPPORT_FIXTURE ?? 'unset'

export const supportFixture = {
  telegramInput: 'https://rocproxy_issue_112_verify.t.me/',
  telegram: 'https://t.me/rocproxy_issue_112_verify',
  qq: 'https://wpa1.qq.com/rocproxy-verify-112?_type=wpa&qidian=true',
  phone: '+1 (202) 555-0142',
  clipboard: '+12025550142',
}

export function supportBuildArgs() {
  if (!['unset', 'configured', 'invalid'].includes(supportFixtureMode)) {
    throw new Error('PROXY_VERIFY_SUPPORT_FIXTURE must be unset, configured, or invalid')
  }

  const values = {
    unset: { telegram: '', qq: '', phone: '' },
    configured: { ...supportFixture, telegram: supportFixture.telegramInput },
    invalid: { telegram: 'https://t.me.invalid/rocproxy_verify', qq: 'https://t.me/rocproxy_verify', phone: 'verify' },
  }[supportFixtureMode]

  return {
    VITE_SUPPORT_TELEGRAM_URL: values.telegram,
    VITE_SUPPORT_QQ_URL: values.qq,
    VITE_SUPPORT_QQ: '',
    VITE_SUPPORT_PHONE: values.phone,
  }
}
