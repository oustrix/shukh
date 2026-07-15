.PHONY: install-hooks

# Ставит git-хуки из scripts/git-hooks в .git/hooks (симлинками).
# Правки версионируемых скриптов вступают в силу сразу, без переустановки.
install-hooks:
	@hooks_dir="$$(git rev-parse --git-path hooks)"; \
	mkdir -p "$$hooks_dir"; \
	for src in scripts/git-hooks/*; do \
		name="$$(basename "$$src")"; \
		chmod +x "$$src"; \
		ln -sf "$$(cd "$$(dirname "$$src")" && pwd)/$$name" "$$hooks_dir/$$name"; \
		echo "installed hook: $$name -> $$hooks_dir/$$name"; \
	done
