import '@fontsource-variable/inter/wght.css';
import './app.css';
import { mount } from 'svelte';
import App from './App.svelte';
import { app } from './lib/state.svelte';

app.init();

mount(App, { target: document.getElementById('app')! });
