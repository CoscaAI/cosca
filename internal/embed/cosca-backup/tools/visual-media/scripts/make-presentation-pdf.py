#!/usr/bin/env python3
"""HornFit — enterprise presentation PDF generator (ReportLab)."""
from reportlab.lib.pagesizes import A4
from reportlab.lib.units import mm
from reportlab.lib.colors import HexColor
from reportlab.lib.enums import TA_CENTER
from reportlab.lib.styles import ParagraphStyle
from reportlab.platypus import (
    BaseDocTemplate, PageTemplate, Frame, Paragraph, Spacer, Table, TableStyle,
    Image, PageBreak, HRFlowable,
)

GREEN = HexColor('#16A34A')
GREEN_DARK = HexColor('#0F5132')
NAVY = HexColor('#0F172A')
SLATE = HexColor('#334155')
MUTED = HexColor('#64748B')
LIGHT = HexColor('#F1F5F9')
BORDER = HexColor('#E2E8F0')
WHITE = HexColor('#FFFFFF')

LOGO = '/home/cosca/Documents/projects/clients/hornfit/apps/web/public/logo-nobg.png'
OUT = '/home/cosca/Documents/projects/clients/hornfit/docs/HornFit-Apresentacao.pdf'
PAGE_W, PAGE_H = A4


def st(name, **kw):
    base = dict(fontName='Helvetica', fontSize=10, leading=14, textColor=SLATE)
    base.update(kw)
    return ParagraphStyle(name, **base)


S_COVER_TITLE = st('coverTitle', fontName='Helvetica-Bold', fontSize=40, leading=46, textColor=NAVY, alignment=TA_CENTER)
S_COVER_SUB = st('coverSub', fontSize=16, leading=22, textColor=MUTED, alignment=TA_CENTER)
S_COVER_TAG = st('coverTag', fontName='Helvetica-Oblique', fontSize=12, leading=18, alignment=TA_CENTER)
S_H2 = st('h2', fontName='Helvetica-Bold', fontSize=13, leading=17, textColor=GREEN_DARK, spaceAfter=3)
S_BODY = st('body', fontSize=10, leading=15, spaceAfter=6)
S_SMALL = st('small', fontSize=8.5, leading=12, textColor=MUTED)
S_CELL = st('cell', fontSize=9.5, leading=13)
S_CELL_T = st('cellT', fontName='Helvetica-Bold', fontSize=10.5, leading=14, textColor=NAVY, spaceAfter=3)
S_NUM = st('num', fontName='Helvetica-Bold', fontSize=20, leading=22, textColor=GREEN, alignment=TA_CENTER)


def section_bar(title):
    inner = Table(
        [[Paragraph(title, st('secT', fontName='Helvetica-Bold', fontSize=16, leading=20, textColor=WHITE))]],
        colWidths=[PAGE_W - 44 * mm],
    )
    inner.setStyle(TableStyle([
        ('BACKGROUND', (0, 0), (-1, -1), GREEN),
        ('LEFTPADDING', (0, 0), (-1, -1), 12),
        ('TOPPADDING', (0, 0), (-1, -1), 10),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 10),
        ('VALIGN', (0, 0), (-1, -1), 'MIDDLE'),
    ]))
    return inner


def card(title, body):
    """Feature card = list of flowables (ReportLab aceita lista em célula)."""
    return [Paragraph(title, S_CELL_T), Paragraph(body, S_CELL)]


def feature_table(cards, cols):
    per_row = cols
    padded = cards + [None] * ((per_row - len(cards) % per_row) % per_row)
    grid = [padded[i:i + per_row] for i in range(0, len(padded), per_row)]
    t = Table(grid, colWidths=[(PAGE_W - 46 * mm) / per_row] * per_row)
    style = [
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('LEFTPADDING', (0, 0), (-1, -1), 8),
        ('RIGHTPADDING', (0, 0), (-1, -1), 8),
        ('TOPPADDING', (0, 0), (-1, -1), 8),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 8),
    ]
    for r in range(len(grid)):
        for c in range(len(grid[r])):
            if grid[r][c] is not None:
                style.append(('BACKGROUND', (c, r), (c, r), LIGHT))
                style.append(('BOX', (c, r), (c, r), 0.6, BORDER))
    t.setStyle(TableStyle(style))
    return t


def kv_table(pairs, col1=48 * mm):
    rows = [[Paragraph(k, st('kvk', fontName='Helvetica-Bold', fontSize=9.5, leading=13, textColor=SLATE)),
             Paragraph(v, st('kvv', fontSize=9.5, leading=13, textColor=NAVY))] for k, v in pairs]
    t = Table(rows, colWidths=[col1, PAGE_W - 46 * mm - col1])
    t.setStyle(TableStyle([
        ('VALIGN', (0, 0), (-1, -1), 'TOP'),
        ('LEFTPADDING', (0, 0), (-1, -1), 6),
        ('RIGHTPADDING', (0, 0), (-1, -1), 6),
        ('TOPPADDING', (0, 0), (-1, -1), 5),
        ('BOTTOMPADDING', (0, 0), (-1, -1), 5),
        ('LINEBELOW', (0, 0), (-1, -2), 0.4, BORDER),
    ]))
    return t


def on_cover(canv, doc):
    pass


def on_page(canv, doc):
    canv.saveState()
    canv.setFillColor(NAVY)
    canv.rect(0, PAGE_H - 6 * mm, PAGE_W, 6 * mm, stroke=0, fill=1)
    canv.setFillColor(GREEN)
    canv.rect(0, PAGE_H - 6 * mm, 40 * mm, 6 * mm, stroke=0, fill=1)
    canv.setFillColor(MUTED)
    canv.setFont('Helvetica', 8)
    canv.drawString(20 * mm, 11 * mm, 'HornFit — Apresentação do Sistema')
    canv.drawRightString(PAGE_W - 20 * mm, 11 * mm, f'Página {doc.page}')
    canv.restoreState()


doc = BaseDocTemplate(OUT, pagesize=A4,
                      leftMargin=20 * mm, rightMargin=20 * mm,
                      topMargin=18 * mm, bottomMargin=18 * mm,
                      title='HornFit — Apresentação', author='HornFit')
doc.addPageTemplates([
    PageTemplate(id='Cover', frames=[Frame(0, 0, PAGE_W, PAGE_H, id='cover')], onPage=on_cover),
    PageTemplate(id='Page', frames=[Frame(20 * mm, 18 * mm, PAGE_W - 40 * mm, PAGE_H - 40 * mm, id='page')], onPage=on_page),
])

story = []

# ══ COVER ══
story.append(Spacer(1, 70 * mm))
try:
    logo = Image(LOGO, width=70 * mm, height=70 * mm)
    logo.hAlign = 'CENTER'
    story.append(logo)
except Exception:
    pass
story.append(Spacer(1, 14 * mm))
story.append(Paragraph('HornFit', S_COVER_TITLE))
story.append(Spacer(1, 4 * mm))
story.append(Paragraph('Assistência Técnica Especializada para Equipamentos de Academia', S_COVER_SUB))
story.append(Spacer(1, 8 * mm))
story.append(HRFlowable(width='60%', thickness=2, color=GREEN, hAlign='CENTER'))
story.append(Spacer(1, 8 * mm))
story.append(Paragraph('Plataforma completa de gestão — do cadastro ao faturamento', S_COVER_TAG))
story.append(Spacer(1, 3 * mm))
story.append(Paragraph('Painel Enterprise · Portal do Cliente · Marketplace · Relatórios', S_COVER_TAG))
story.append(Spacer(1, 40 * mm))
story.append(Paragraph('Documento de apresentação · 2026', S_SMALL))
story.append(PageBreak())

# ══ 1. VISÃO GERAL ══
story.append(section_bar('Visão Geral'))
story.append(Spacer(1, 6 * mm))
story.append(Paragraph('HornFit é um sistema de gestão único para empresas de manutenção de equipamentos de academia e condomínios. '
                       'Em uma única plataforma, a operação completa: cadastro de clientes, ordens de serviço, contratos, faturamento, '
                       'estoque, CRM e atendimento em tempo real.', S_BODY))
story.append(Spacer(1, 4 * mm))
story.append(feature_table([
    card('Gestão única', 'Sem multi-tenant: um painel, uma operação, dados consolidados.'),
    card('Tempo real', 'Chamados e notificações via WebSocket — status sempre atualizado.'),
    card('Enterprise', 'RBAC por perfil, auditoria, relatórios e rastreio de erros.'),
    card('pt-BR', 'Interface em português para o cliente final, código em inglês.'),
], 2))
story.append(Spacer(1, 8 * mm))
story.append(Paragraph('Números da plataforma', S_H2))
story.append(feature_table([
    card('25+', 'endpoints REST + WebSocket'),
    card('22', 'entidades de domínio'),
    card('6', 'perfis de acesso (RBAC)'),
    card('20+', 'telas no painel e portal'),
], 4))
story.append(PageBreak())

# ══ 2. MÓDULOS ══
story.append(section_bar('Módulos do Sistema'))
story.append(Spacer(1, 6 * mm))
story.append(feature_table([
    card('Clientes', 'PF/PJ, endereços, contatos, equipamentos e histórico.'),
    card('Ordens de Serviço', 'Fluxo completo com linha do tempo, prioridades e custos.'),
    card('Contratos', 'Academia/condomínio, coberturas, SLA e renovação.'),
    card('Faturamento', 'Faturas, pagamentos, inadimplência e 2ª via.'),
    card('Estoque', 'Peças, fornecedores, movimentações e alerta de mínimo.'),
    card('CRM', 'Leads, pipeline, interações e propostas.'),
    card('Chamados', 'Helpdesk em tempo real (WebSocket) com histórico.'),
    card('FAQ & Auto-ajuda', 'Central de ajuda pesquisável para o cliente.'),
    card('Marketplace', 'Loja de peças com carrinho, checkout e pedidos.'),
    card('Relatórios', 'Financeiro, operacional, estoque e exportação CSV.'),
    card('Páginas / CMS', 'Page builder para landing e marketplace.'),
    card('Erros & Observabilidade', 'Rastreio de erros com painel de debug.'),
], 3))
story.append(PageBreak())

# ══ 3. ARQUITETURA ══
story.append(section_bar('Arquitetura & Stack'))
story.append(Spacer(1, 6 * mm))
story.append(kv_table([
    ('Frontend', 'Next.js 15 (App Router) · React 19 · Tailwind CSS · design system próprio'),
    ('Backend', 'NestJS 11 · REST + WebSocket · Prisma ORM'),
    ('Banco de dados', 'PostgreSQL 16 · Redis 7 (cache/filas)'),
    ('Realtime', 'Socket.IO — chamados e notificações ao vivo'),
    ('Storage', 'MinIO (S3-compatible) para uploads e mídia'),
    ('Infra', 'Docker Compose · imagens multi-stage · CI/CD (GitHub Actions)'),
    ('Monorepo', 'pnpm workspaces + Turborepo'),
]))
story.append(Spacer(1, 6 * mm))
story.append(Paragraph('Pipeline de feature (Cosca Engineering Intelligence Matrix)', S_H2))
story.append(Paragraph('Product Analysis → Arquitetura → Banco → API → Backend → Frontend → Segurança → Teste → Performance → Observabilidade → Review → Aprovação.', S_BODY))
story.append(PageBreak())

# ══ 4. EXPERIÊNCIA ══
story.append(section_bar('Experiência Enterprise'))
story.append(Spacer(1, 6 * mm))
story.append(Paragraph('Painel Enterprise', S_H2))
story.append(Paragraph('Dashboard com KPIs e gráficos reais (tendência de receita, faturas por status, OS por status, top clientes), '
                       'sidebar colapsável por fluxo, busca, filtros, ações em massa e detalhes ricos em drawer.', S_BODY))
story.append(Spacer(1, 4 * mm))
story.append(Paragraph('Portal do Cliente', S_H2))
story.append(Paragraph('Visão de minhas OS, contratos, faturas e chamados — com chat em tempo real e central de ajuda.', S_BODY))
story.append(Spacer(1, 4 * mm))
story.append(Paragraph('Marketplace', S_H2))
story.append(Paragraph('Catálogo com desconto, carrinho persistente, checkout com validação de estoque e pedidos.', S_BODY))
story.append(Spacer(1, 6 * mm))
story.append(Paragraph('Design & Identidade', S_H2))
story.append(feature_table([
    card('Design System', 'Tokens semânticos, 14 componentes base, consistência global.'),
    card('Cores operacionais', 'Verde = operacional · vermelho = incidente · âmbar = manutenção.'),
    card('Acessibilidade', 'Contraste 4.5:1, foco visível, reduced-motion, teclado.'),
], 3))
story.append(PageBreak())

# ══ 5. QUALIDADE & SEGURANÇA ══
story.append(section_bar('Qualidade, Segurança & Padrões'))
story.append(Spacer(1, 6 * mm))
story.append(kv_table([
    ('Segurança', 'Auth JWT + refresh rotativo · RBAC por perfil · senha com bcrypt'),
    ('Testes', 'Esteira verde: typecheck, testes unitários e build em CI'),
    ('Observabilidade', 'Rastreio de erros persistido + painel de debug'),
    ('Padrões Cosca', 'Desenvolvido sob a Cosca Engineering Intelligence Matrix (16 stacks)'),
]))
story.append(Spacer(1, 6 * mm))
story.append(Paragraph('Roadmap & Extensões', S_H2))
story.append(Paragraph('• Integração de pagamento real (Stripe/Pix) · • Upload e mídia via MinIO · • Testes E2E (Playwright) · '
                       '• Deploy em VPS com domínio · • Relatórios avançados e BI.', S_BODY))
story.append(PageBreak())

# ══ 6. ACESSO ══
story.append(section_bar('Acesso & Demonstração'))
story.append(Spacer(1, 8 * mm))
story.append(kv_table([
    ('Painel Enterprise', 'admin@hornfit.com  ·  Admin@123'),
    ('Portal do Cliente', 'cliente@hornfit.com  ·  Cliente@123'),
]))
story.append(Spacer(1, 8 * mm))
story.append(Paragraph('HornFit — um produto pensado como sistema, entregue como experiência.',
                       st('h2', fontName='Helvetica-Bold', fontSize=13, leading=17, textColor=GREEN, alignment=TA_CENTER)))
story.append(Spacer(1, 4 * mm))
story.append(Paragraph('Documento gerado automaticamente pela Cosca Visual Media Engine.', S_SMALL))

doc.build(story)
print(f'PDF gerado: {OUT}')
