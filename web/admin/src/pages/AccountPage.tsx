import { Stack } from '@mantine/core'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '../components/PageHeader'
import { PasskeysCard } from '../components/PasskeysCard'
import { TwoFactorCard } from '../components/TwoFactorCard'
import { TokensCard } from '../components/TokensCard'
export default function AccountPage() {
  const { t } = useTranslation()
  return <><PageHeader title={t('passkeys.account')} subtitle={t('passkeys.hint')} /><Stack><PasskeysCard /><TwoFactorCard /><TokensCard /></Stack></>
}
