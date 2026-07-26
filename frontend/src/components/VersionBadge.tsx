import { useState, useEffect } from "react"
import { Tag, PlusCircle, History } from "lucide-react"
import { Button } from "./ui/button"
import Swal from "sweetalert2"
import { GetAppVersion, GetVersionHistory, PublishVersion } from "../../bindings/changeme/macrosservice"

interface AppVersion {
  version: string
  date: string
}

interface VersionEntry {
  version: string
  date: string
  changelog: string
}

export function VersionBadge() {
  const [version, setVersion] = useState<AppVersion | null>(null)
  const [history, setHistory] = useState<VersionEntry[]>([])
  const [showHistory, setShowHistory] = useState(false)

  useEffect(() => {
    GetAppVersion().then((v) => setVersion(v ?? null))
  }, [])

  const handlePublish = async () => {
    const { value: changelog } = await Swal.fire({
      title: "Yeni versiyon yayınla",
      html: `
        <div style="text-align: left; font-size: 14px;">
          <label style="display: block; margin-bottom: 4px; font-weight: 500;">Versiyon türü</label>
          <select id="bump-type" class="swal2-select" style="width: 100%; padding: 8px; border-radius: 6px; border: 1px solid #ccc; margin-bottom: 12px;">
            <option value="patch">Patch (0.0.1 → 0.0.2)</option>
            <option value="minor">Minor (0.0.1 → 0.1.0)</option>
            <option value="major">Major (0.0.1 → 1.0.0)</option>
          </select>
          <label style="display: block; margin-bottom: 4px; font-weight: 500;">Değişiklik notları</label>
          <textarea id="changelog" class="swal2-textarea" rows="5" style="width: 100%; padding: 8px; border-radius: 6px; border: 1px solid #ccc; resize: vertical;" placeholder="Bu versiyonda ne değişti?"></textarea>
        </div>
      `,
      focusConfirm: false,
      showCancelButton: true,
      confirmButtonText: "Yayınla",
      cancelButtonText: "İptal",
      confirmButtonColor: "#2563eb",
      preConfirm: () => {
        const bumpType = (document.getElementById("bump-type") as HTMLSelectElement).value
        const changelog = (document.getElementById("changelog") as HTMLTextAreaElement).value
        if (!changelog.trim()) {
          Swal.showValidationMessage("Değişiklik notu gerekli")
          return false
        }
        return { bumpType, changelog: changelog.trim() }
      },
      background: document.documentElement.classList.contains("dark") ? "#1e1e2e" : "#ffffff",
      color: document.documentElement.classList.contains("dark") ? "#cdd6f4" : "#1e1e2e",
    })

    if (!changelog) return

    try {
      const newVer = await PublishVersion(changelog.changelog, changelog.bumpType)
      setVersion(newVer ?? null)
      Swal.fire({
        icon: "success",
        title: `v${newVer?.version} yayınlandı!`,
        timer: 2000,
        showConfirmButton: false,
        background: document.documentElement.classList.contains("dark") ? "#1e1e2e" : "#ffffff",
        color: document.documentElement.classList.contains("dark") ? "#cdd6f4" : "#1e1e2e",
        position: "bottom-end",
        toast: true,
      })
    } catch (err: any) {
      Swal.fire({
        icon: "error",
        title: "Hata",
        text: err?.message || "Yayınlanamadı.",
      })
    }
  }

  const openHistory = async () => {
    const hist = await GetVersionHistory()
    setHistory(hist ?? [])
    setShowHistory(true)
  }

  return (
    <>
      <div className="flex items-center gap-1">
        <Button variant="ghost" size="sm" onClick={openHistory} className="gap-1 text-xs" title="Versiyon geçmişi">
          <Tag size={12} />
          v{version?.version || "?"}
        </Button>
        <Button variant="ghost" size="icon" onClick={handlePublish} title="Yeni versiyon yayınla">
          <PlusCircle size={15} />
        </Button>
      </div>

      {/* History Modal */}
      {showHistory && (
        <div className="fixed inset-0 z-50 flex items-start justify-center pt-12 bg-black/40">
          <div className="w-full max-w-lg bg-white dark:bg-zinc-800 rounded-lg shadow-xl border border-zinc-200 dark:border-zinc-700 max-h-[70vh] flex flex-col">
            <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
              <div className="flex items-center gap-2">
                <History size={16} className="text-zinc-500" />
                <h2 className="text-sm font-semibold">Versiyon Geçmişi</h2>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setShowHistory(false)}>
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              </Button>
            </div>
            <div className="flex-1 overflow-auto p-2">
              {history.length === 0 ? (
                <div className="text-center py-8 text-sm text-zinc-400">Henüz versiyon yok.</div>
              ) : (
                <div className="space-y-2">
                  {history.map((entry) => (
                    <div key={entry.version} className="px-3 py-2 rounded-md border border-zinc-200 dark:border-zinc-700">
                      <div className="flex items-center justify-between">
                        <span className="text-sm font-semibold text-blue-600 dark:text-blue-400">
                          v{entry.version}
                        </span>
                        <span className="text-xs text-zinc-400">{new Date(entry.date).toLocaleDateString("tr-TR", { year: "numeric", month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" })}</span>
                      </div>
                      <p className="text-xs text-zinc-600 dark:text-zinc-400 mt-1 whitespace-pre-wrap">{entry.changelog}</p>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
