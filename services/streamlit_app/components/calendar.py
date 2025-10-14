"""Componentes para el calendario"""

import streamlit as st
import calendar
from datetime import datetime
from typing import List, Dict
from utils.events import get_events_for_day

def change_month(delta_months):
    """Callback para cambiar el mes/año"""
    current_date = st.session_state.current_date

    # Calcular nueva fecha
    new_month = current_date.month + delta_months
    new_year = current_date.year

    # Ajustar año si es necesario
    while new_month > 12:
        new_month -= 12
        new_year += 1
    while new_month < 1:
        new_month += 12
        new_year -= 1

    st.session_state.current_date = current_date.replace(year=new_year, month=new_month)
    st.session_state.selected_date = None

def change_year(delta_years):
    """Callback para cambiar el año"""
    st.session_state.current_date = st.session_state.current_date.replace(
        year=st.session_state.current_date.year + delta_years
    )
    st.session_state.selected_date = None

# Nombres de meses en español
SPANISH_MONTHS = {
    1: "Enero", 2: "Febrero", 3: "Marzo", 4: "Abril", 5: "Mayo", 6: "Junio",
    7: "Julio", 8: "Agosto", 9: "Septiembre", 10: "Octubre", 11: "Noviembre", 12: "Diciembre"
}

def get_previous_month_name():
    """Obtener el nombre del mes anterior en español"""
    current_date = st.session_state.current_date
    if current_date.month == 1:
        return SPANISH_MONTHS[12]  # Diciembre del año anterior
    else:
        return SPANISH_MONTHS[current_date.month - 1]

def get_next_month_name():
    """Obtener el nombre del mes siguiente en español"""
    current_date = st.session_state.current_date
    if current_date.month == 12:
        return SPANISH_MONTHS[1]  # Enero del año siguiente
    else:
        return SPANISH_MONTHS[current_date.month + 1]

def navigate_calendar():
    """Controles de navegación del calendario"""
    col1, col2, col3, col4, col5 = st.columns([1, 1, 2, 1, 1])

    with col1:
        st.button(f"◀◀ {st.session_state.current_date.year - 1}", on_click=change_year, args=(-1,))

    with col2:
        prev_month = get_previous_month_name()
        st.button(f"◀ {prev_month}", on_click=change_month, args=(-1,))

    with col3:
        # Selector de mes y año
        current_year = st.session_state.current_date.year
        current_month = st.session_state.current_date.month

        selected_year = st.selectbox(
            "Año",
            range(current_year - 10, current_year + 11),
            index=10,
            label_visibility="collapsed"
        )

        selected_month = st.selectbox(
            "Mes",
            list(range(1, 13)),
            format_func=lambda x: SPANISH_MONTHS[x],
            index=current_month - 1,
            label_visibility="collapsed"
        )

        if selected_year != current_year or selected_month != current_month:
            st.session_state.current_date = st.session_state.current_date.replace(
                year=selected_year, month=selected_month
            )
            st.session_state.selected_date = None

    with col4:
        next_month = get_next_month_name()
        st.button(f"{next_month} ▶", on_click=change_month, args=(1,))

    with col5:
        st.button(f"{st.session_state.current_date.year + 1} ▶▶", on_click=change_year, args=(1,))

def render_calendar():
    """Renderizar calendario mensual navegable con eventos"""
    current_month_name = SPANISH_MONTHS[st.session_state.current_date.month]
    st.subheader(f"{current_month_name} {st.session_state.current_date.year}")

    # Controles de navegación
    navigate_calendar()

    # Obtener días del mes
    cal = calendar.monthcalendar(
        st.session_state.current_date.year,
        st.session_state.current_date.month
    )

    # Nombres de los días
    days = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom']

    # Header de días
    cols = st.columns(7)
    for i, day in enumerate(days):
        with cols[i]:
            st.write(f"**{day}**")

    # Crear el calendario con claves únicas
    for week_num, week in enumerate(cal):
        cols = st.columns(7)
        for i, day in enumerate(week):
            with cols[i]:
                if day != 0:
                    current_day = datetime(
                        st.session_state.current_date.year,
                        st.session_state.current_date.month,
                        day
                    )

                    # Verificar si hay eventos en este día
                    day_events = get_events_for_day(current_day)

                    # Determinar si es hoy
                    is_today = current_day.date() == datetime.now().date()

                    # Determinar si está seleccionado
                    is_selected = (
                        st.session_state.selected_date and
                        st.session_state.selected_date.date() == current_day.date()
                    )

                    # Texto del botón
                    button_text = f"{day}"
                    if day_events:
                        button_text += f"\n🔵 {len(day_events)}"

                    # Color del botón
                    button_type = "primary" if is_selected else "secondary"

                    # 🔑 CLAVE ÚNICA para cada botón
                    unique_key = f"day_{st.session_state.current_date.year}_{st.session_state.current_date.month}_{day}"

                    # Botón clickeable para el día
                    if st.button(
                        button_text,
                        key=unique_key,  # Clave única basada en fecha
                        use_container_width=True,
                        type=button_type
                    ):
                        st.session_state.selected_date = current_day
                        st.session_state.form_submitted = False  # Reset form state
                        st.rerun()

                else:
                    # 🔑 Días vacíos también necesitan claves únicas
                    empty_key = f"empty_{week_num}_{i}"
                    st.button("", key=empty_key, disabled=True, use_container_width=True)