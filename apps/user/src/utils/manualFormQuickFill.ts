export interface QuickFillField {
  key: string
  type: string
  options: string[]
}

const agreementPattern = /(agreement|consent|terms|accept|协议|協議|声明|聲明|确认|確認|同意)/i
const emailPattern = /(email|e-mail|邮箱|信箱)/i

export const resolveManualQuickFillValue = (
  field: QuickFillField,
  label: string,
  placeholder: string,
  email: string,
): { handled: boolean; value?: string | string[] } => {
  const searchable = [field.key, label, placeholder].join(' ')
  const isAgreement = agreementPattern.test(searchable)
  const isEmail = field.type === 'email' || emailPattern.test([field.key, label].join(' '))

  if (isEmail && email.trim()) {
    return { handled: true, value: email.trim() }
  }
  if (field.type === 'checkbox' && field.options.length > 0 && isAgreement) {
    return { handled: true, value: [...field.options] }
  }
  if ((field.type === 'text' || field.type === 'textarea') && isAgreement) {
    return { handled: true, value: placeholder || label }
  }
  return { handled: false }
}
