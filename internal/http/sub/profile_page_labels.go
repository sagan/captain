package sub

import (
	"fmt"
	"strings"
)

func bytesLabel(n int64) string {
	if n >= 1<<30 {
		return fmt.Sprintf("%.2f GiB", float64(n)/(1<<30))
	}
	if n >= 1<<20 {
		return fmt.Sprintf("%.2f MiB", float64(n)/(1<<20))
	}
	return fmt.Sprintf("%d B", n)
}
func profilePageLabels(accept string) [3]string {
	lang := strings.ToLower(strings.Split(accept, ",")[0])
	switch {
	case strings.HasPrefix(lang, "zh-tw"), strings.HasPrefix(lang, "zh-hk"), strings.HasPrefix(lang, "zh-hant"):
		return [3]string{"流量用量", "到期日期", "選擇客戶端格式。請保管好此私人訂閱連結；臨時連結每次成功開啟此頁或下載訂閱都會消耗一次。"}
	case strings.HasPrefix(lang, "zh"):
		return [3]string{"流量用量", "到期日期", "选择客户端格式。请保管好此私人订阅链接；临时链接每次成功打开此页或下载订阅都会消耗一次。"}
	case strings.HasPrefix(lang, "ja"):
		return [3]string{"使用量", "有効期限", "クライアント形式を選択してください。この個人用リンクは非公開にしてください。一時リンクはページ表示またはダウンロードごとに1回使用します。"}
	case strings.HasPrefix(lang, "ru"):
		return [3]string{"Трафик", "Срок действия", "Выберите формат клиента. Сохраняйте ссылку в тайне. Открытие страницы или загрузка расходует одно использование временной ссылки."}
	case strings.HasPrefix(lang, "ko"):
		return [3]string{"사용량", "만료일", "클라이언트 형식을 선택하세요. 이 개인 링크를 안전하게 보관하세요. 임시 링크는 페이지 표시 또는 다운로드마다 한 번 사용됩니다."}
	default:
		return [3]string{"Usage", "Expiry", "Choose your client format. Keep this private subscription link confidential. Each successful page view or download uses one temporary-link allowance."}
	}
}
