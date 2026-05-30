;;; sesh.el --- Default sesh-open-project / sesh-close-project hooks  -*- lexical-binding: t; -*-

;; Copyright (C) Ian Duggan.
;; SPDX-License-Identifier: MIT

;;; Commentary:

;; Default policy for the sesh v0.4 emacs plugin. Source this file from your
;; init.el (or modmacs / spacemacs / doom equivalent) to get a sensible
;; tab-bar-based workspace per sesh project.
;;
;; sesh dispatches mechanism-only -- it calls (sesh-open-project name cwd
;; files) and (sesh-close-project name) via emacsclient and gets out of the
;; way. The functions below are policy: open a tab-bar tab named after the
;; project, cd to its cwd, optionally open requested files. Replace freely
;; with your own choices (perspective.el, project.el, etc.).

;;; Code:

(require 'tab-bar)

;;;###autoload
(defun sesh-open-project (name cwd &optional files)
  "Open sesh project NAME at CWD; optionally open FILES.
Creates a tab-bar tab named NAME, sets `default-directory' to CWD,
and opens each path in FILES via `find-file'. Customize freely -- sesh
calls this whenever `sesh up' runs."
  (tab-bar-mode 1)
  (tab-bar-new-tab)
  (tab-bar-rename-tab name)
  (when (and cwd (file-directory-p cwd))
    (setq default-directory (file-name-as-directory cwd)))
  (dolist (f files)
    (find-file f)))

;;;###autoload
(defun sesh-close-project (name)
  "Close sesh project NAME's tab-bar tab if present.
No-op when the tab is missing -- `sesh down' calls this best-effort."
  (when (bound-and-true-p tab-bar-mode)
    (ignore-errors (tab-bar-close-tab-by-name name))))

(provide 'sesh)
;;; sesh.el ends here
