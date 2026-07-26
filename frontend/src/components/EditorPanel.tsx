import { useEffect, useRef, useState } from "react"
import { EditorView, basicSetup } from "codemirror"
import { EditorState } from "@codemirror/state"
import { python } from "@codemirror/lang-python"
import { oneDark } from "@codemirror/theme-one-dark"
import Swal from "sweetalert2"
import { Save, FileCode, Undo2 } from "lucide-react"
import { Button } from "./ui/button"
import { ReadFile, SaveFile } from "../../bindings/changeme/macrosservice"
import { VersionHistory } from "./VersionHistory"

interface EditorPanelProps {
  filePath: string | null
}

export function EditorPanel({ filePath }: EditorPanelProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const [saving, setSaving] = useState(false)
  const [dirty, setDirty] = useState(false)
  const [fileName, setFileName] = useState("")
  const [viewingVersionLabel, setViewingVersionLabel] = useState<string | null>(null)
  const currentFilePath = useRef<string | null>(null)

  const createEditor = (content: string, isDark: boolean, readOnly: boolean) => {
    if (!editorRef.current) return

    if (viewRef.current) {
      viewRef.current.destroy()
    }

    const extensions = [
      basicSetup,
      python(),
      EditorView.editable.of(!readOnly),
      EditorView.updateListener.of((update) => {
        if (update.docChanged && !readOnly) setDirty(true)
      }),
      EditorView.theme({
        "&": { height: "100%" },
        ".cm-scroller": { overflow: "auto" },
      }),
    ]

    if (isDark) {
      extensions.push(oneDark)
    }

    const state = EditorState.create({
      doc: content,
      extensions,
    })

    viewRef.current = new EditorView({
      state,
      parent: editorRef.current,
    })
  }

  const loadFile = (path: string, versionContent?: string, versionLabel?: string) => {
    setFileName(path.split("/").pop() || path)
    setDirty(false)
    currentFilePath.current = path

    if (versionContent !== undefined) {
      // Viewing a version
      const isDark = document.documentElement.classList.contains("dark")
      createEditor(versionContent, isDark, true)
      setViewingVersionLabel(versionLabel || null)
    } else {
      // Normal file load
      setViewingVersionLabel(null)
      ReadFile(path).then((content) => {
        const isDark = document.documentElement.classList.contains("dark")
        createEditor(content, isDark, false)
      })
    }
  }

  useEffect(() => {
    if (!filePath) return
    loadFile(filePath)

    return () => {
      // cleanup handled in createEditor
    }
  }, [filePath])

  // Watch for theme changes
  useEffect(() => {
    if (!filePath) return

    const observer = new MutationObserver(() => {
      if (!viewRef.current) return
      const content = viewRef.current.state.doc.toString()
      const isDark = document.documentElement.classList.contains("dark")
      createEditor(content, isDark, viewingVersionLabel !== null)
    })

    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] })
    return () => observer.disconnect()
  }, [filePath, viewingVersionLabel])

  const handleViewVersion = (content: string, label: string) => {
    if (filePath) {
      loadFile(filePath, content, label)
    }
  }

  const handleBackToCurrent = () => {
    if (filePath) {
      loadFile(filePath)
    }
  }

  const handleSave = async () => {
    if (!filePath || !viewRef.current || viewingVersionLabel) return
    setSaving(true)
    try {
      const content = viewRef.current.state.doc.toString()
      await SaveFile(filePath, content)
      setDirty(false)
      Swal.fire({
        icon: "success",
        title: "Kaydedildi",
        text: `${fileName} başarıyla kaydedildi.`,
        timer: 1500,
        showConfirmButton: false,
        position: "bottom-end",
        toast: true,
        background: document.documentElement.classList.contains("dark") ? "#1e1e2e" : "#ffffff",
        color: document.documentElement.classList.contains("dark") ? "#cdd6f4" : "#1e1e2e",
      })
    } catch (err: any) {
      Swal.fire({
        icon: "error",
        title: "Hata",
        text: err?.message || "Dosya kaydedilemedi.",
      })
    } finally {
      setSaving(false)
    }
  }

  if (!filePath) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-zinc-400 dark:text-zinc-500 gap-3">
        <FileCode size={48} strokeWidth={1} />
        <p className="text-lg">Bir dosya seç</p>
        <p className="text-sm">Soldaki ağaçtan bir Python dosyası seçerek düzenlemeye başlayın.</p>
      </div>
    )
  }

  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-zinc-200 dark:border-zinc-700 bg-zinc-50 dark:bg-zinc-800/50">
        <div className="flex items-center gap-2 min-w-0">
          <FileCode size={16} className="text-blue-400 shrink-0" />
          <span className="text-sm font-medium truncate">{fileName}</span>
          {dirty && !viewingVersionLabel && (
            <span className="text-xs text-amber-500 dark:text-amber-400 shrink-0">kaydedilmemiş</span>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <VersionHistory filePath={filePath} onViewVersion={handleViewVersion} />
          <Button
            size="sm"
            onClick={handleSave}
            disabled={!dirty || saving || !!viewingVersionLabel}
            className="gap-1"
          >
            <Save size={14} />
            {saving ? "Kaydediliyor..." : "Kaydet"}
          </Button>
        </div>
      </div>

      {/* Version banner */}
      {viewingVersionLabel && (
        <div className="flex items-center justify-between px-4 py-2 bg-amber-50 dark:bg-amber-900/20 border-b border-amber-200 dark:border-amber-800">
          <span className="text-xs text-amber-700 dark:text-amber-300">
            📜 Eski versiyon görüntüleniyor: <strong>{viewingVersionLabel}</strong>
          </span>
          <Button variant="ghost" size="sm" onClick={handleBackToCurrent} className="gap-1 text-xs">
            <Undo2 size={12} />
            Güncel dosyaya dön
          </Button>
        </div>
      )}

      {/* Editor */}
      <div ref={editorRef} className="flex-1 overflow-hidden" />
    </div>
  )
}
