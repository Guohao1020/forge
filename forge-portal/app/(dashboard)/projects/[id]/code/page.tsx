"use client";

import { useEffect, useState, useCallback } from "react";
import { useParams } from "next/navigation";
import { Code2, Loader2, GitBranch, AlertCircle, FileCode } from "lucide-react";
import { getCodeTree, getCodeFile, listBranches } from "@/lib/code";
import type { Branch } from "@/lib/code";
import { BranchSelector } from "@/components/code-browser/branch-selector";
import { FileBreadcrumb } from "@/components/code-browser/file-breadcrumb";
import { RepoFileTree } from "@/components/code-browser/repo-file-tree";
import { ShikiCodeViewer } from "@/components/code-preview/shiki-code-viewer";

export default function CodeBrowserPage() {
  const params = useParams();
  const projectId = params.id as string;

  const [branches, setBranches] = useState<Branch[]>([]);
  const [currentBranch, setCurrentBranch] = useState("");
  const [files, setFiles] = useState<string[]>([]);
  const [selectedPath, setSelectedPath] = useState("");
  const [fileContent, setFileContent] = useState("");
  const [treeLoading, setTreeLoading] = useState(true);
  const [fileLoading, setFileLoading] = useState(false);
  const [branchLoading, setBranchLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Load branches
  useEffect(() => {
    let cancelled = false;
    setBranchLoading(true);
    listBranches(projectId)
      .then((data) => {
        if (cancelled) return;
        setBranches(data);
        if (data.length > 0 && !currentBranch) {
          const main = data.find(
            (b) => b.name === "main" || b.name === "master"
          );
          setCurrentBranch(main?.name || data[0].name);
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err.message);
      })
      .finally(() => {
        if (!cancelled) setBranchLoading(false);
      });
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [projectId]);

  // Load file tree when branch changes
  const loadTree = useCallback(
    async (branch: string) => {
      if (!branch) return;
      setTreeLoading(true);
      setError(null);
      try {
        const data = await getCodeTree(projectId, branch);
        setFiles(data.files || []);
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : "加载文件树失败";
        setError(message);
        setFiles([]);
      } finally {
        setTreeLoading(false);
      }
    },
    [projectId]
  );

  useEffect(() => {
    if (currentBranch) {
      setSelectedPath("");
      setFileContent("");
      loadTree(currentBranch);
    }
  }, [currentBranch, loadTree]);

  // Load file content
  const handleFileSelect = useCallback(
    async (path: string) => {
      setSelectedPath(path);
      setFileLoading(true);
      try {
        const data = await getCodeFile(projectId, path, currentBranch);
        setFileContent(data.content || "");
      } catch {
        setFileContent("// 加载文件内容失败");
      } finally {
        setFileLoading(false);
      }
    },
    [projectId, currentBranch]
  );

  // Breadcrumb navigation: select a directory path (clear file)
  const handleBreadcrumbNavigate = useCallback((path: string) => {
    setSelectedPath("");
    setFileContent("");
    // Could expand the tree to this path in future
  }, []);

  const handleBranchChange = useCallback((branch: string) => {
    setCurrentBranch(branch);
  }, []);

  // Error / empty state: no repo connected
  if (error && files.length === 0 && !treeLoading) {
    return (
      <div>
        <h1 className="text-2xl font-semibold tracking-tight mb-6">
          代码浏览
        </h1>
        <div className="flex flex-col items-center justify-center py-20 rounded-xl border border-border bg-card">
          <div className="w-12 h-12 rounded-xl flex items-center justify-center mb-3 bg-primary/10">
            <AlertCircle className="h-6 w-6 text-primary" />
          </div>
          <h3 className="text-base font-medium mb-1">无法加载代码</h3>
          <p className="text-sm text-muted-foreground max-w-md text-center">
            请先在项目设置中关联 GitHub 仓库，或检查仓库访问权限
          </p>
          <p className="text-xs text-white/20 mt-2">{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full -m-6">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-white/10 bg-white/[0.02] shrink-0">
        <div className="flex items-center gap-3">
          <Code2 className="h-5 w-5 text-[#8B5CF6]" />
          <h1 className="text-lg font-semibold">代码浏览</h1>
        </div>
        <div className="flex items-center gap-2">
          <BranchSelector
            branches={branches}
            currentBranch={currentBranch}
            loading={branchLoading}
            onChange={handleBranchChange}
          />
        </div>
      </div>

      {/* Split pane: tree + code */}
      <div className="flex flex-1 overflow-hidden">
        {/* Left: file tree */}
        <div className="w-[260px] shrink-0 border-r border-white/10 overflow-y-auto bg-white/[0.01]">
          {/* 仓库信息栏 */}
          {!treeLoading && files.length > 0 && (
            <div className="flex items-center gap-2 px-3 py-2 border-b border-white/10 text-xs text-white/40">
              <GitBranch className="h-3 w-3 text-[#8B5CF6]" />
              <span className="font-mono text-white/60">{currentBranch}</span>
              <span className="mx-1">·</span>
              <FileCode className="h-3 w-3" />
              <span>{files.length} 个文件</span>
            </div>
          )}
          {treeLoading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="h-5 w-5 animate-spin text-white/20" />
            </div>
          ) : (
            <RepoFileTree
              files={files}
              selectedPath={selectedPath}
              onSelect={handleFileSelect}
            />
          )}
        </div>

        {/* Right: code viewer */}
        <div className="flex-1 flex flex-col overflow-hidden">
          {selectedPath && (
            <FileBreadcrumb
              path={selectedPath}
              onNavigate={handleBreadcrumbNavigate}
            />
          )}
          <div className="flex-1 overflow-auto">
            {fileLoading ? (
              <div className="flex items-center justify-center h-full">
                <Loader2 className="h-5 w-5 animate-spin text-white/20" />
              </div>
            ) : selectedPath ? (
              <ShikiCodeViewer
                content={fileContent}
                fileName={selectedPath}
              />
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-white/20 gap-2">
                <Code2 className="h-8 w-8" />
                <span className="text-sm">
                  从左侧文件树选择文件查看代码
                </span>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
