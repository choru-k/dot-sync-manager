" Sample .vimrc for E2E testing
set number
set relativenumber
set tabstop=4
set shiftwidth=4
set expandtab
set smarttab
set softtabstop=4
set autoindent
set copyindent
set smartindent
set background=dark
set showmatch
set ignorecase
set smartcase
set incsearch
set hlsearch
set ruler
set laststatus=2
set visualbell
set ttyfast
set spell

" Color scheme
if has('syntax')
    syntax on
endif

" Key mappings
nnoremap <C-h> <C-w>h
nnoremap <C-j> <C-w>j
nnoremap <C-k> <C-w>k
nnoremap <C-l> <C-w>l

" Filetype specific settings
if has('autocmd')
    filetype plugin indent on
    autocmd FileType go setlocal tabstop=4 shiftwidth=4 expandtab
    autocmd FileType python setlocal tabstop=4 shiftwidth=4 expandtab
    autocmd FileType javascript setlocal tabstop=2 shiftwidth=2 expandtab
endif
