"use client";

import { useState } from "react";
import {
  ChevronRight,
  ChevronDown,
  Folder,
  FolderOpen,
  File,
  FileCode,
  FileText,
  FileJson,
  FileImage,
} from "lucide-react";
import { cn } from "@/lib/utils";

interface RepoFileTreeProps {
  files: string[];
  selectedPath?: string;
  onSelect: (path: string) => void;
}

interface TreeNode {
  name: string;
  fullPath: string;
  isDir: boolean;
  children: TreeNode[];
}

const CODE_EXTENSIONS = new Set([
  "ts", "tsx", "js", "jsx", "go", "py", "java", "rs", "rb", "php",
  "vue", "svelte", "css", "scss", "html", "htm", "sh", "bash",
  "c", "cpp", "h", "cs", "swift", "kt",
]);

const TEXT_EXTENSIONS = new Set(["md", "txt", "yml", "yaml", "toml", "cfg", "ini", "env"]);
const JSON_EXTENSIONS = new Set(["json", "jsonc", "json5"]);
const IMAGE_EXTENSIONS = new Set(["png", "jpg", "jpeg", "gif", "svg", "webp", "ico"]);

function getFileIcon(name: string) {
  const ext = name.split(".").pop()?.toLowerCase() || "";
  if (CODE_EXTENSIONS.has(ext)) return FileCode;
  if (TEXT_EXTENSIONS.has(ext)) return FileText;
  if (JSON_EXTENSIONS.has(ext)) return FileJson;
  if (IMAGE_EXTENSIONS.has(ext)) return FileImage;
  if (name.toLowerCase().includes("dockerfile")) return FileCode;
  return File;
}

function buildTree(files: string[]): TreeNode[] {
  const root: TreeNode[] = [];

  // Pre-compute which paths are directories:
  // A path is a directory if any other path starts with it + "/"
  const dirSet = new Set<string>();
  for (const filePath of files) {
    const parts = filePath.split("/");
    let prefix = "";
    for (let i = 0; i < parts.length - 1; i++) {
      prefix = prefix ? `${prefix}/${parts[i]}` : parts[i];
      dirSet.add(prefix);
    }
  }
  // Also check: if a path exists in pathSet AND is a prefix of another path, it's a dir
  for (const filePath of files) {
    for (const other of files) {
      if (other !== filePath && other.startsWith(filePath + "/")) {
        dirSet.add(filePath);
        break;
      }
    }
  }

  for (const filePath of files) {
    const parts = filePath.split("/");
    let current = root;
    let pathSoFar = "";

    for (let i = 0; i < parts.length; i++) {
      const part = parts[i];
      pathSoFar = pathSoFar ? `${pathSoFar}/${part}` : part;
      const isLast = i === parts.length - 1;
      const isDir = !isLast || dirSet.has(pathSoFar);

      let node = current.find((n) => n.name === part);
      if (!node) {
        node = {
          name: part,
          fullPath: pathSoFar,
          isDir,
          children: [],
        };
        current.push(node);
      } else if (isDir && !node.isDir) {
        // Upgrade to directory if we discover it's a prefix
        node.isDir = true;
      }
      current = node.children;
    }
  }

  // Sort: folders first, then files alphabetically
  function sortTree(nodes: TreeNode[]): TreeNode[] {
    nodes.sort((a, b) => {
      if (a.isDir !== b.isDir) return a.isDir ? -1 : 1;
      return a.name.localeCompare(b.name);
    });
    for (const node of nodes) {
      if (node.isDir) sortTree(node.children);
    }
    return nodes;
  }

  return sortTree(root);
}

function TreeNodeItem({
  node,
  selectedPath,
  onSelect,
  depth = 0,
}: {
  node: TreeNode;
  selectedPath?: string;
  onSelect: (path: string) => void;
  depth?: number;
}) {
  const [expanded, setExpanded] = useState(depth < 1);

  if (node.isDir) {
    return (
      <div>
        <button
          onClick={() => setExpanded(!expanded)}
          className="flex items-center gap-1.5 w-full px-2 py-1 text-xs text-muted-foreground hover:text-foreground/70 hover:bg-muted/50 transition-colors"
          style={{ paddingLeft: `${depth * 12 + 8}px` }}
        >
          {expanded ? (
            <ChevronDown className="h-3 w-3 shrink-0 text-muted-foreground/60" />
          ) : (
            <ChevronRight className="h-3 w-3 shrink-0 text-muted-foreground/60" />
          )}
          {expanded ? (
            <FolderOpen className="h-3.5 w-3.5 shrink-0 text-accent/70" />
          ) : (
            <Folder className="h-3.5 w-3.5 shrink-0 text-accent/70" />
          )}
          <span className="truncate">{node.name}</span>
        </button>
        {expanded &&
          node.children.map((child) => (
            <TreeNodeItem
              key={child.fullPath}
              node={child}
              selectedPath={selectedPath}
              onSelect={onSelect}
              depth={depth + 1}
            />
          ))}
      </div>
    );
  }

  const isSelected = selectedPath === node.fullPath;
  const Icon = getFileIcon(node.name);

  return (
    <button
      onClick={() => onSelect(node.fullPath)}
      className={cn(
        "flex items-center gap-1.5 w-full px-2 py-1 text-xs transition-colors",
        isSelected
          ? "bg-accent/10 text-foreground"
          : "text-muted-foreground hover:text-foreground/80 hover:bg-muted/50"
      )}
      style={{ paddingLeft: `${depth * 12 + 8}px` }}
    >
      <span className="w-3 shrink-0" />
      <Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground/60" />
      <span className="truncate">{node.name}</span>
    </button>
  );
}

export function RepoFileTree({ files, selectedPath, onSelect }: RepoFileTreeProps) {
  const tree = buildTree(files);

  if (files.length === 0) {
    return (
      <div className="flex items-center justify-center h-full text-muted-foreground/40 text-xs py-8">
        暂无文件
      </div>
    );
  }

  return (
    <div className="overflow-y-auto py-1">
      {tree.map((node) => (
        <TreeNodeItem
          key={node.fullPath}
          node={node}
          selectedPath={selectedPath}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
