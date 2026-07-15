import { Routes, Route } from 'react-router-dom'
import { ROOM_ROUTE, TABLE_ROUTE } from './routes'
import { Join } from './ui/screens/Join'
import { Lobby } from './ui/screens/Lobby'
import { Table } from './ui/screens/Table'

export function App() {
  return (
    <Routes>
      <Route path="/" element={<Join />} />
      <Route path={ROOM_ROUTE} element={<Lobby />} />
      <Route path={TABLE_ROUTE} element={<Table />} />
    </Routes>
  )
}
