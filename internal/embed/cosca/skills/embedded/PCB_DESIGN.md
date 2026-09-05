# PCB Design — KiCad Enterprise

> **Version**: 1.0.0 | **Tool**: KiCad 8 | **Scope**: Schematics, layout, routing, gerber, BOM

## Workflow

```bash
kicad-cli sch export pdf    --output schematic.pdf    project.kicad_sch
kicad-cli pcb export gerber --output gerber/          project.kicad_pcb
kicad-cli pcb export drill  --output gerber/          project.kicad_pcb
kicad-cli pcb export bom    --output bom.csv          project.kicad_pcb
```

## Design Rules

- **Trace width**: 0.25mm for signal, 0.5mm for power (>500mA), 1mm for >2A
- **Clearance**: 0.2mm minimum between traces
- **Via**: 0.6mm hole, 1.0mm pad for 2-layer; 0.3mm/0.6mm for 4-layer
- **Ground plane**: solid copper pour on both sides, stitched with vias every 10mm
- **Decoupling**: 100nF ceramic within 5mm of EVERY IC power pin + 10µF bulk per rail
- **ESD protection**: TVS diodes on all external connectors (USB, GPIO, power)
- **Silkscreen**: component outlines, polarity markers (diode/IC pin 1), test points labeled

## Layer Stack (4-layer)

```
L1: Signal + Power (top)
L2: Ground plane
L3: Power plane
L4: Signal (bottom)
```

## Gerber Validation

```bash
gerbv gerber/*.g*           # Visual inspection
# Online: DFM check at jlcpcb.com/pcb-quote or pcbway.com
```

## Security

- No backdoor test points exposed on production boards
- JTAG/SWD headers disabled in production (or secured with lock bits)
- Unique ID burned into each board (eFuse/OTP) for supply chain traceability
