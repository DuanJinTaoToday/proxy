import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

export { accountLoadFailed, currentAccount, hasSession, installSession, loadAccount, register, sessionPhone, signIn, signOut } from './session'
export { maskPhone, normalizePhone, safeRedirect } from './rules'
export { default as AuthForm } from './AuthForm.vue'
export { default as AccountMenu } from './AccountMenu.vue'

// 本模块的中英文文案，由应用层统一注册。
export const messages = { 'zh-CN': zhCN, en }

export { currentOperator, hasOperatorPermission, hasOperatorSession, installOperatorSession, loadOperator, safeOperatorRedirect, signInOperator, signOutOperator } from './operatorSession'
export { default as OperatorMenu } from './OperatorMenu.vue'
