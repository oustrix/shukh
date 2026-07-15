import { Routes, Route } from 'react-router-dom'
import { Join } from './ui/screens/Join'
import { Lobby } from './ui/screens/Lobby'
import { Table } from './ui/screens/Table'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Join />} />
      <Route path="/room/:code" element={<Lobby />} />
      <Route path="/room/:code/table" element={<Table />} />
    </Routes>
  )
}
