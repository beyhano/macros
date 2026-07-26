import { useState, useEffect } from "react"
import { History, Eye, RotateCcw, X, Clock, FileText, Hash } from "lucide-react"
import { Button } from "./ui/button"
import Swal from "sweetalert2"
import { GetVersions, GetVersionContent, RestoreVersion } from "../../bindings/changeme/macrosservice"

interface VersionInfo {
  versionId: string
  modifiedAt: string
  size: number
  sha1: string
}

interface VersionHistoryProps {
  filePath: string | null
  onViewVersion: (content: string, label: string) => void
}

export function VersionHistory({ filePath, onViewVersion }: VersionHistoryProps) {
  const [open, setOpen] = useState(false)
  const [versions, setVersions] = useState<VersionInfo[]>([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    if (!open || !filePath) return
    setLoading(true)
    GetVersions(filePath).then((v) => {
      setVersions(v ?? [])
      setLoading(false)
    })
  }, [open, filePath])

  const handleView = async (ver: VersionInfo) => {
    if (!filePath) return
    try {
      const content = await GetVersionContent(filePath, ver.versionId)
      onViewVersion(content, `${filePath.split("/").pop()} — ${ver.modifiedAt}`)
      setOpen(false)
    } catch (err: any) {
      Swal.fire({ icon: "error", title: "Hata", text: err?.message || "Versiyon okunamadı." })
    }
  }

  const handleRestore = async (ver: VersionInfo) => {
    if (!filePath) return
    const result = await Swal.fire({
      title: "Emin misin?",
      text: `Dosya ${ver.modifiedAt} tarihli versiyona geri alınacak.`,
      icon: "warning",
      showCancelButton: true,
      confirmButtonText: "Geri al",
      cancelButtonText: "İptal",
      confirmButtonColor: "#dc2626",
      background: document.documentElement.classList.contains("dark") ? "#1e1e2e" : "#ffffff",
      color: document.documentElement.classList.contains("dark") ? "#cdd6f4" : "#1e1e2e",
    })
    if (!result.isConfirmed) return

    try {
      await RestoreVersion(filePath, ver.versionId)
      Swal.fire({
        icon: "success",
        title: "Geri alındı",
        text: `Versiyon ${ver.modifiedAt} geri yüklendi. Sayfayı yenileyin.`,
        timer: 2000,
        showConfirmButton: false,
      }).then(() => {
        window.location.reload()
      })
    } catch (err: any) {
      Swal.fire({ icon: "error", title: "Hata", text: err?.message || "Geri alınamadı." })
    }
  }

  return (
    <>
      <Button
        variant="ghost"
        size="sm"
        onClick={() => setOpen(true)}
        disabled={!filePath}
        title="Versiyon geçmişi"
        className="gap-1"
      >
        <History size={14} />
        Versiyonlar
      </Button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-start justify-center pt-12 bg-black/40">
          <div className="w-full max-w-lg bg-white dark:bg-zinc-800 rounded-lg shadow-xl border border-zinc-200 dark:border-zinc-700 max-h-[70vh] flex flex-col">
            {/* Header */}
            <div className="flex items-center justify-between px-4 py-3 border-b border-zinc-200 dark:border-zinc-700">
              <div className="flex items-center gap-2">
                <History size={16} className="text-zinc-500" />
                <h2 className="text-sm font-semibold">Versiyon Geçmişi</h2>
              </div>
              <Button variant="ghost" size="icon" onClick={() => setOpen(false)}>
                <X size={16} />
              </Button>
            </div>

            {/* Body */}
            <div className="flex-1 overflow-auto p-2">
              {loading ? (
                <div className="text-center py-8 text-sm text-zinc-400">Yükleniyor...</div>
              ) : versions.length === 0 ? (
                <div className="text-center py-8 text-sm text-zinc-400">
                  Henüz versiyon yok.
                  <br />
                  <span className="text-xs">Dosyayı kaydettiğinizde otomatik yedek oluşur.</span>
                </div>
              ) : (
                <div className="space-y-1">
                  {versions.map((ver) => (
                    <div
                      key={ver.versionId}
                      className="flex items-center justify-between px-3 py-2 rounded-md hover:bg-zinc-100 dark:hover:bg-zinc-700/50 text-sm"
                    >
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 text-xs text-zinc-600 dark:text-zinc-400">
                          <Clock size={12} />
                          <span>{ver.modifiedAt}</span>
                        </div>
                        <div className="flex items-center gap-3 mt-1 text-xs text-zinc-400">
                          <span className="flex items-center gap-1">
                            <FileText size={10} />
                            {ver.size} B
                          </span>
                          <span className="flex items-center gap-1 truncate">
                            <Hash size={10} />
                            {ver.sha1.substring(0, 12)}…
                          </span>
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0 ml-2">
                        <Button variant="ghost" size="sm" onClick={() => handleView(ver)} title="Görüntüle">
                          <Eye size={14} />
                        </Button>
                        <Button variant="ghost" size="sm" onClick={() => handleRestore(ver)} title="Geri al">
                          <RotateCcw size={14} />
                        </Button>
                      </div>
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
