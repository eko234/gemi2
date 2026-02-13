declare-option str gmtempprefixorsomething /tmp/gmkak

declare-option str gmchatoutfifo
declare-option str gmchatinhost localhost
declare-option str gmchatinport 6969
declare-option str gmcursel


declare-option str gmprogram gm
declare-option bool gmstarted false

define-command opengm -hidden -override %{
  eval -try-client tools %{
    edit -fifo %opt{gmchatoutfifo} chatgm
    set buffer filetype markdown
  }
}

define-command ge -override %{
  nop %sh{
    printf 'out:/dev/null;in:@clear\n' | nc -N "$kak_opt_gmchatinhost" "$kak_opt_gmchatinport"
  }
}

define-command gm -override -params 0.. %{
  evaluate-commands %sh{
    if [ "$kak_opt_gmstarted" = false ]; then
      outfifo=$(mktemp -u "${kak_opt_gmtempprefixorsomething}XXXXXXXX")
      mkfifo $outfifo
      echo "set-option global gmchatoutfifo '$outfifo'"
      echo "set-option global gmstarted true"
    fi
  }

  eval %sh{
    sel=$(printf "%s" "$kak_selection" | sed ':a;N;$!ba;s/\n/\\n/g')
    echo "set-option global gmcursel %{$sel}"
  }

  opengm

  nop %sh{
    msg="$*"
    if [ ${#kak_opt_gmcursel} -gt 1 ]; then
      printf 'out:%s;in:%s\n' "$kak_opt_gmchatoutfifo" "$msg $kak_opt_gmcursel" | nc -N "$kak_opt_gmchatinhost" "$kak_opt_gmchatinport"
    else
      printf 'out:%s;in:%s\n' "$kak_opt_gmchatoutfifo" "$msg" | nc -N "$kak_opt_gmchatinhost" "$kak_opt_gmchatinport"
    fi
  }
}
