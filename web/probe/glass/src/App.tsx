import { useEffect, useState } from 'react'
import { Icon } from '@iconify/react'
import { t } from '@shared/i18n'
import { snapshot } from '@shared/captain'
import { Alert, AlertAction, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Toaster } from '@/components/ui/sonner'
import Background from '@/components/Background'
import Footer from '@/components/Footer'
import Header from '@/components/Header'
import LoadingCover from '@/components/LoadingCover'
import Provider from '@/components/Provider'
import Transition from '@/components/Transition'
import { useBox } from '@/hooks/useBox'
import { useVisitorPageAudit } from '@/hooks/useVisitorAudit'
import RouterView from '@/RouterView'
import { useApp } from '@/stores/app'
import { destroyInitManager, initApp, retryInitApp } from '@/utils/init'


export default function App() {
  useVisitorPageAudit()
  const loading = useApp(s => s.loading)
  const connectionError = useApp(s => s.connectionError)
  const disablePageAnimation = useApp(s => s.disablePageAnimation)
  const snap = useBox(snapshot)
  const [isRetryingConnection, setIsRetryingConnection] = useState(false)

  async function retryConnection(): Promise<void> {
    if (isRetryingConnection)
      return
    setIsRetryingConnection(true)
    try {
      const recovered = await retryInitApp()
      if (recovered)
        window.$message?.success(t('连接已恢复。'))
      else
        window.$message?.error(t('仍无法连接服务器，请稍后再试。'))
    }
    catch (error) {
      console.error('[App] Connection retry failed:', error)
      window.$message?.error(t('重试失败，请稍后再试。'))
    }
    finally {
      setIsRetryingConnection(false)
    }
  }

  useEffect(() => {
    initApp().catch((error) => {
      console.error('[App] Initialization failed:', error)
    })
    return () => destroyInitManager()
  }, [])

  return (
    <Provider>
      <Background />
      <Transition
        css={!disablePageAnimation}
        enterActiveClass="transition-opacity duration-200 ease-out"
        enterFromClass="opacity-0"
        enterToClass="opacity-100"
        leaveActiveClass="transition-opacity duration-300 ease-in"
        leaveFromClass="opacity-100"
        leaveToClass="opacity-0"
      >
        {loading && <LoadingCover />}
      </Transition>
      <Header />
      <Transition
        css={!disablePageAnimation}
        enterActiveClass="transition-all duration-300 ease-out"
        enterFromClass="opacity-0 translate-y-2"
        enterToClass="opacity-100 translate-y-0"
      >
        {!loading && (
          <div className="app-shell">
            <main className="min-h-screen overflow-hidden">
              {connectionError && (
                <div className="relative z-10 mx-auto max-w-[1280px] px-4 pt-4">
                  <Alert variant="destructive" className="!pr-28 border-none bg-destructive/10 backdrop-blur-xs rounded-md">
                    <Icon icon="tabler:plug-connected-x" />
                    <AlertTitle>{t('服务连接失败')}</AlertTitle>
                    <AlertDescription>{t('连接服务器失败，请检查网络后重试。')}</AlertDescription>
                    <AlertAction className="top-1/2 -translate-y-1/2">
                      <Button size="sm" variant="outline" disabled={isRetryingConnection} onClick={() => void retryConnection()}>
                        <Icon icon={isRetryingConnection ? 'tabler:loader-2' : 'tabler:refresh'} className={isRetryingConnection ? 'animate-spin' : undefined} />
                        {isRetryingConnection ? t('重试中') : t('重试')}
                      </Button>
                    </AlertAction>
                  </Alert>
                </div>
              )}
              {snap && (
                <div className="max-w-[1280px] mx-auto">
                  <RouterView />
                </div>
              )}
            </main>
            <Footer />
          </div>
        )}
      </Transition>
      <Toaster richColors closeButton position="top-center" />
    </Provider>
  )
}
