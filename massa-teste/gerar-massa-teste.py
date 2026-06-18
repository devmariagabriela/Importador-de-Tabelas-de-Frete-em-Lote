#!/usr/bin/env python3
"""Gera CSV de tabelas de frete com erros propositais para teste de validação."""

import csv
import os
import random

CIDADES = [
    "SAO PAULO",
    "RIO DE JANEIRO",
    "BELO HORIZONTE",
    "CURITIBA",
    "PORTO ALEGRE",
    "FLORIANOPOLIS",
    "CAMPINAS",
    "SANTOS",
    "MANAUS",
    "BELEM",
    "RECIFE",
    "SALVADOR",
    "GOIANIA",
    "BRASILIA",
    "VITORIA",
    "NATAL",
    "FORTALEZA",
    "LONDRINA",
]

FAIXAS_PESO = [(0, 5), (5, 10), (10, 30), (30, 50), (50, 100), (100, 500), (500, 1000)]


def gerar_linha_valida():
    origem = random.choice(CIDADES)
    destino = random.choice([cidade for cidade in CIDADES if cidade != origem])
    peso_min, peso_max = random.choice(FAIXAS_PESO)
    valor = round(random.uniform(15, 500), 2)
    return [origem, destino, str(peso_min), str(peso_max), str(valor)]


def gerar_linha_campo_vazio():
    linha = gerar_linha_valida()
    linha[random.choice([0, 1])] = ""
    return linha


def gerar_linha_peso_invertido():
    origem = random.choice(CIDADES)
    destino = random.choice([cidade for cidade in CIDADES if cidade != origem])
    peso_min = random.randint(20, 100)
    peso_max = random.randint(1, peso_min - 1)
    valor = round(random.uniform(15, 500), 2)
    return [origem, destino, str(peso_min), str(peso_max), str(valor)]


def gerar_linha_valor_invalido():
    linha = gerar_linha_valida()
    linha[4] = str(random.choice([0, -1, -5.5, -100]))
    return linha


def gerar_linha_peso_negativo():
    linha = gerar_linha_valida()
    linha[2] = str(random.randint(-10, -1))
    return linha


def main():
    os.chdir(os.path.dirname(os.path.abspath(__file__)))

    total = 5000
    erros_qtd = int(total * 0.2)
    validas_qtd = total - erros_qtd

    linhas = [gerar_linha_valida() for _ in range(validas_qtd)]
    geradores_erro = [
        gerar_linha_campo_vazio,
        gerar_linha_peso_invertido,
        gerar_linha_valor_invalido,
        gerar_linha_peso_negativo,
    ]
    linhas.extend(random.choice(geradores_erro)() for _ in range(erros_qtd))
    duplicatas = random.sample(linhas[:validas_qtd], min(100, validas_qtd))
    linhas.extend(duplicatas)
    random.shuffle(linhas)

    arquivo = "tabela_frete_teste.csv"
    with open(arquivo, "w", newline="") as handle:
        writer = csv.writer(handle)
        writer.writerow(["origem", "destino", "peso_min", "peso_max", "valor"])
        writer.writerows(linhas)

    print(f"Gerado: {arquivo}")
    print(f"Total: {len(linhas)} linhas")
    print(f"~{validas_qtd} válidas, ~{erros_qtd} com erro, ~{len(duplicatas)} duplicatas")


if __name__ == "__main__":
    main()
