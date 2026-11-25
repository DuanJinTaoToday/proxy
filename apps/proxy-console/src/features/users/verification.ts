// 运营协助账号操作（重置密码、发送恢复邮件）前核实租户身份的方式。核实记录只用于本次操作回执。
export const verificationMethods = ['phone', 'transfer', 'license'] as const
