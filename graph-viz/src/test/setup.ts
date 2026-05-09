import '@testing-library/jest-dom';

// Mock window.confirm for tests
(globalThis as unknown as { confirm: () => boolean }).confirm = () => true;

